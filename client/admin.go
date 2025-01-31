// Copyright 2017 fatedier, fatedier@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/gorilla/mux"

	"github.com/fatedier/frp/assets"
	utilnet "github.com/fatedier/frp/pkg/util/net"
)

var (
	httpServerReadTimeout  = 60 * time.Second
	httpServerWriteTimeout = 60 * time.Second
	router                 = mux.NewRouter()
)

func (svr *Service) GetAdminRouter() *mux.Router {
	return router
}

func (svr *Service) AddAdminRouter(path, methods string, f func(http.ResponseWriter, *http.Request)) {
	router.HandleFunc(path, f).Methods(methods)
}

func (svr *Service) RunAdminServer(address string) (err error) {

	router.HandleFunc("/healthz", svr.healthz)

	// debug
	if svr.cfg.PprofEnable {
		router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		router.HandleFunc("/debug/pprof/profile", pprof.Profile)
		router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		router.HandleFunc("/debug/pprof/trace", pprof.Trace)
		router.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
	}

	subRouter := router.NewRoute().Subrouter()
	user, passwd := svr.cfg.AdminUser, svr.cfg.AdminPwd
	subRouter.Use(utilnet.NewHTTPAuthMiddleware(user, passwd).SetAuthFailDelay(200 * time.Millisecond).Middleware)

	// api, see admin_api.go
	subRouter.HandleFunc("/api/reload", svr.apiReload).Methods("GET")
	subRouter.HandleFunc("/api/stop", svr.apiStop).Methods("POST")
	subRouter.HandleFunc("/api/status", svr.apiStatus).Methods("GET")
	subRouter.HandleFunc("/api/config", svr.apiGetConfig).Methods("GET")
	subRouter.HandleFunc("/api/config", svr.apiPutConfig).Methods("PUT")

	// view
	subRouter.Handle("/favicon.ico", http.FileServer(assets.FileSystem)).Methods("GET")
	subRouter.PathPrefix("/static/").Handler(utilnet.MakeHTTPGzipHandler(http.StripPrefix("/static/", http.FileServer(assets.FileSystem)))).Methods("GET")
	subRouter.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/", http.StatusMovedPermanently)
	})

	// 创建 CORS 处理函数
	corsHandler := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// 设置允许的源
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("origin"))

			// 设置允许的方法
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")

			// 设置允许的请求头
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// 继续处理请求
			h.ServeHTTP(w, r)
		})
	}

	server := &http.Server{
		Addr:         address,
		Handler:      corsHandler(router),
		ReadTimeout:  httpServerReadTimeout,
		WriteTimeout: httpServerWriteTimeout,
	}
	if address == "" {
		address = ":http"
	}
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	go func() {
		_ = server.Serve(ln)
	}()
	return
}
