package multipipelinehs

import (
	"flag"
	"net/http"

	"github.com/tncheng/multipipelinehs/config"
	"github.com/tncheng/multipipelinehs/log"
)

func Init() {
	flag.Parse()
	log.Setup()
	config.Configuration.Load()
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 1000
}
