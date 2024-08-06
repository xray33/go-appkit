/*
Copyright © 2024 Acronis International GmbH.

Released under MIT license.
*/

package restapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRoutePath(t *testing.T) {
	tests := []struct {
		Name          string
		RoutePathStr  string
		WantRoutePath RoutePath
		WantErrStr    string
	}{
		{
			Name:         "only spaces",
			RoutePathStr: "  ",
			WantErrStr:   "path is missing",
		},
		{
			Name:         "prefixed match, not started with /",
			RoutePathStr: "foobar",
			WantErrStr:   "path should be started with \"/\" in case of prefixed matching",
		},
		{
			Name:          "prefixed match, ok",
			RoutePathStr:  "/",
			WantRoutePath: RoutePath{Raw: "/", NormalizedPath: "/"},
		},
		{
			Name:          "prefixed match, ok",
			RoutePathStr:  "////",
			WantRoutePath: RoutePath{Raw: "////", NormalizedPath: "/"},
		},
		{
			Name:          "prefixed match, ok",
			RoutePathStr:  "/foobar///",
			WantRoutePath: RoutePath{Raw: "/foobar///", NormalizedPath: "/foobar/"},
		},
		{
			Name:         "exact match, not started with /",
			RoutePathStr: "=",
			WantErrStr:   "path should be started with \"/\" in case of exact matching",
		},
		{
			Name:         "exact match, not started with /",
			RoutePathStr: "= foobar",
			WantErrStr:   "path should be started with \"/\" in case of exact matching",
		},
		{
			Name:          "exact match, ok",
			RoutePathStr:  "= ///a/./b/..///",
			WantRoutePath: RoutePath{Raw: "= ///a/./b/..///", NormalizedPath: "/a/", ExactMatch: true},
		},
		{
			Name:         "forward match, not started with /",
			RoutePathStr: "^~",
			WantErrStr:   "path should be started with \"/\" in case of forward matching",
		},
		{
			Name:         "forward match, not started with /",
			RoutePathStr: "^~ foobar",
			WantErrStr:   "path should be started with \"/\" in case of forward matching",
		},
		{
			Name:          "forward match, ok",
			RoutePathStr:  "^~ ///a/./b/..///",
			WantRoutePath: RoutePath{Raw: "^~ ///a/./b/..///", NormalizedPath: "/a/", ForwardMatch: true},
		},
		{
			Name:         "regexp match, not started with /",
			RoutePathStr: "~",
			WantErrStr:   "regular expression is missing",
		},
		{
			Name:         "regexp match, parsing err",
			RoutePathStr: "~ (sdf!* ",
			WantErrStr:   "error parsing regexp: missing closing ): `(sdf!*`",
		},
		{
			Name:         "regexp match, ok",
			RoutePathStr: "~ /tenants/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/users",
			WantRoutePath: RoutePath{
				Raw:            "~ /tenants/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/users",
				NormalizedPath: "",
				RegExpPath:     regexp.MustCompile("/tenants/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/users"),
			},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.Name, func(t *testing.T) {
			routePath, err := ParseRoutePath(tt.RoutePathStr)
			if tt.WantErrStr != "" {
				require.EqualError(t, err, tt.WantErrStr)
				return
			}
			require.Equal(t, tt.WantRoutePath, routePath)
		})
	}
}

func TestRoutesManager_SearchMatchedRouteForRequest(t *testing.T) {
	predefinedRoutes := []RouteConfig{
		{Path: mustParseRoutePath("/"), Methods: []string{http.MethodGet}},
		{Path: mustParseRoutePath("/a")},
		{Path: mustParseRoutePath("= /aa"), Methods: []string{http.MethodGet}},
		{Path: mustParseRoutePath("= /aa"), Methods: []string{http.MethodPost}},
		{Path: mustParseRoutePath("/aaa")},
		{Path: mustParseRoutePath("/bbb")},
		{Path: mustParseRoutePath("/eee/fff/ggg")},
		{Path: mustParseRoutePath("/same/route/as/excluded")},
		{Path: mustParseRoutePath("/same/route/as-excluded-except-method"), Methods: []string{http.MethodGet}},
		{Path: mustParseRoutePath("/static")},
		{Path: mustParseRoutePath("^~ /media")},
		{Path: mustParseRoutePath("~ ^/(static|media)")},
		{Path: mustParseRoutePath("~ ^/content/(files|folders)$")},
		{Path: mustParseRoutePath("~ ^/static/(javascript|images)$")},
		{Path: mustParseRoutePath("~ (?i)^/admin/")},
	}

	excludedRoutesInConfig := []RouteConfig{
		{Path: mustParseRoutePath("/aaa/bbb")},
		{Path: mustParseRoutePath("/eee/fff")},
		{Path: mustParseRoutePath("/same/route/as/excluded")},
		{Path: mustParseRoutePath("/same/route/as-excluded-except-method"), Methods: []string{http.MethodPost}},
		{Path: mustParseRoutePath("~ ^/content/files/(png|jpeg)")},
	}
	tests := []struct {
		Routes         []RouteConfig
		ExcludedRoutes []RouteConfig
		Req            *http.Request
		WantFound      bool
		WantFoundRoute RouteConfig
	}{
		{
			Req:       httptest.NewRequest(http.MethodPost, "/", nil),
			WantFound: false,
		},
		{
			Req:       httptest.NewRequest(http.MethodGet, "/c", nil),
			WantFound: false,
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{mustParseRoutePath("/"), []string{http.MethodGet}},
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/aa", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("= /aa"), Methods: []string{http.MethodGet}},
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/aa/", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("/a")},
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/aaa", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("/aaa")},
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/aaaa", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("/aaa")},
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/AAAA", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("/"), Methods: []string{http.MethodGet}},
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/aab", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("/a")},
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/bb", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("/"), Methods: []string{http.MethodGet}},
		},
		{
			Req:       httptest.NewRequest(http.MethodPost, "/bb", nil),
			Routes:    predefinedRoutes,
			WantFound: false,
		},
		{
			Req:            httptest.NewRequest(http.MethodPost, "/bbb", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("/bbb")},
		},
		{
			Req:            httptest.NewRequest(http.MethodPost, "/static", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("~ ^/(static|media)")},
		},
		{
			Req:            httptest.NewRequest(http.MethodPost, "/media/", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("^~ /media")},
		},
		{
			Req:            httptest.NewRequest(http.MethodPost, "/static/images", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("~ ^/(static|media)")},
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/STATIC/images", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("/"), Methods: []string{http.MethodGet}},
		},
		{
			Req:            httptest.NewRequest(http.MethodPost, "/ADMIN/login", nil),
			Routes:         predefinedRoutes,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("~ (?i)^/admin/")},
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/aaa", nil),
			Routes:         predefinedRoutes,
			ExcludedRoutes: excludedRoutesInConfig,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("/aaa")},
		},
		{
			Req:            httptest.NewRequest(http.MethodPost, "/eee/fff/ggg", nil),
			Routes:         predefinedRoutes,
			ExcludedRoutes: excludedRoutesInConfig,
			WantFound:      false,
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/aaa/bbb", nil),
			Routes:         predefinedRoutes,
			ExcludedRoutes: excludedRoutesInConfig,
			WantFound:      false,
		},
		{
			Req:            httptest.NewRequest(http.MethodPost, "/static/images", nil),
			Routes:         predefinedRoutes,
			ExcludedRoutes: excludedRoutesInConfig,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("~ ^/(static|media)")},
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/content/files/png", nil),
			Routes:         predefinedRoutes,
			ExcludedRoutes: excludedRoutesInConfig,
			WantFound:      false,
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/same/route/as/excluded", nil),
			Routes:         predefinedRoutes,
			ExcludedRoutes: excludedRoutesInConfig,
			WantFound:      false,
		},
		{
			Req:            httptest.NewRequest(http.MethodGet, "/same/route/as-excluded-except-method", nil),
			Routes:         predefinedRoutes,
			ExcludedRoutes: excludedRoutesInConfig,
			WantFound:      true,
			WantFoundRoute: RouteConfig{Path: mustParseRoutePath("/same/route/as-excluded-except-method"), Methods: []string{http.MethodGet}},
		},
		{
			Req:            httptest.NewRequest(http.MethodPost, "/same/route/as-excluded-except-method", nil),
			Routes:         predefinedRoutes,
			ExcludedRoutes: excludedRoutesInConfig,
			WantFound:      false,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.Req.Method+"_"+tt.Req.URL.Path, func(t *testing.T) {
			var limRoutes []Route
			for _, r := range tt.Routes {
				limRoutes = append(limRoutes, Route{Path: r.Path, Methods: r.Methods})
			}
			for _, r := range tt.ExcludedRoutes {
				limRoutes = append(limRoutes, Route{Path: r.Path, Methods: r.Methods, Excluded: true})
			}
			gotLimRoute, found := NewRoutesManager(limRoutes).SearchMatchedRouteForRequest(tt.Req)
			gotRoute := RouteConfig{gotLimRoute.Path, gotLimRoute.Methods}
			require.Equal(t, tt.WantFound, found)
			if tt.WantFound {
				require.Equal(t, tt.WantFoundRoute, gotRoute)
			}
		})
	}
}

func TestNormalizeURLPath(t *testing.T) {
	tests := []struct {
		path    string
		wantRes string
	}{
		{path: "", wantRes: "/"},
		{path: "/", wantRes: "/"},
		{path: "/foo", wantRes: "/foo"},
		{path: "/foo/", wantRes: "/foo/"},
		{path: "////", wantRes: "/"},
		{path: "/..//../../", wantRes: "/"},
		{path: "/foo/../bar/./qux/", wantRes: "/bar/qux/"},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(fmt.Sprintf("normalizing %q", tt.path), func(t *testing.T) {
			require.Equal(t, tt.wantRes, NormalizeURLPath(tt.path))
		})
	}
}

func mustParseRoutePath(s string) RoutePath {
	rp, err := ParseRoutePath(s)
	if err != nil {
		panic(err)
	}
	return rp
}