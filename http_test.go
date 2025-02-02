package main

import (
	"fmt"
	"glint/ast"
	brohttp "glint/brohttp"
	"glint/config"
	"glint/logger"
	"net/url"
	"testing"

	"github.com/logrusorgru/aurora"
)

func TestCheckPayloadNormal(t *testing.T) {
	var err error
	Spider := brohttp.Spider{}
	logger.DebugEnable(true)
	var taskconfig config.TaskConfig
	taskconfig.Proxy = "127.0.0.1:7777"
	Spider.Init(taskconfig)
	defer Spider.Close()
	tab, err := brohttp.NewTab(&Spider)
	tab.ReqMode = "GET"
	// Spider.PostData = []byte("txtName=ecrrgaowle&mtxMessage=Crawl&user_token=873c33f5ece8c8308071890f478ded0b")
	tab.Url, err = url.Parse("https://challenge-1121.intigriti.io/challenge/index.php?s=111")

	if err != nil {
		t.Errorf(err.Error())
	}
	playload := "<ScRiPt>e6X1fe54</sCrIpT>"
	htmls, _ := tab.CheckPayloadLocation(playload)
	for _, v := range htmls {
		ast.SearchInputInResponse(playload, v)
		// fmt.Println(aurora.Red(OCC))
	}

	htmls, _ = tab.CheckPayloadLocation(playload)
	for _, v := range htmls {
		OCC := ast.SearchInputInResponse(playload, v)
		fmt.Println(aurora.Red(OCC))
	}
}
