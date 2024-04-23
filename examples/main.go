package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/viper"
	_ "github.com/tusdesign/viper-kubernetes"
)

func main() {
	v := viper.New()

	err := v.AddRemoteProvider("configmap", "default", "lemon/lemon.toml")
	if err != nil {
		log.Printf("Failed to add remote provider %v", err)
	}
	v.SetConfigType("toml")
	err = v.ReadRemoteConfig()
	p := v.Get("System.Listen")
	fmt.Println(p)
	if err != nil {
		log.Printf("Failed to read remote config %v", err)
	}

	err = v.WatchRemoteConfigOnChannel()
	if err != nil {
		log.Printf("Failed to watch remote config change %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		s := v.GetString("app.name")
		log.Printf("Name: %s", s)
		_, err := writer.Write([]byte(s))
		if err != nil {
			log.Printf("Failed to write response %v", err)
		}
	})
	log.Fatal(http.ListenAndServe(":80", mux))
}
