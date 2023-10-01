package main

import (
	"fmt"
	"net/http"
	"path"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

func listenPXE() {
	go func() {
		fs := http.FileServer(http.Dir("./static/"))

		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/pxe/", pxeHandler)
		http.Handle("/static/", http.StripPrefix("/static/", fs))
		err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", config.PxePort), nil)
		if err != nil {
			log.Panic(err)
		}
	}()
}

func pxeHandler(w http.ResponseWriter, r *http.Request) {
	configName := path.Base(r.RequestURI)
	log.Debugf("Request PXE config: %s", configName)

	pxe, err := kClient.V1alpha1().PXE().Get(configName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal error!"))

		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(pxe.Spec.Data))
}

// func staticHandler(w http.ResponseWriter, r *http.Request) {
// 	configName := path.Base(r.RequestURI)
// 	log.Debugf("Request PXE config: %s", configName)

// 	pxe, err := kClient.V1alpha1().PXE().Get(configName)
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		w.Write([]byte("Internal error!"))

// 		return
// 	}

// 	w.WriteHeader(http.StatusOK)
// 	w.Write([]byte(pxe.Spec.Data))
// }

// fs := http.FileServer(http.Dir("/home/bob/static"))
// http.Handle("/static/", http.StripPrefix("/static", fs))
