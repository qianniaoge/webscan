package plugin

import (
	"context"
	"encoding/json"
	"glint/brohttp"
	"glint/logger"
	"glint/util"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
)

type Plugin struct {
	Taskid       int                //任务id，只有插入数据库的时候使用
	PluginName   string             //插件名
	MaxPoolCount int                //协程池最大并发数
	Callbacks    []PluginCallback   //扫描插件函数
	Pool         *ants.PoolWithFunc //
	threadwg     sync.WaitGroup     //同步线程
	ScanResult   []*util.ScanResult
	mu           sync.Mutex
	Spider       *brohttp.Spider
	InstallDB    bool //是否插入数据库
	Ctx          *context.Context
	Cancel       *context.CancelFunc
	Timeout      time.Duration
}

type PluginOption struct {
	PluginWg   *sync.WaitGroup
	Progress   *int
	IsSocket   bool
	Data       map[string][]interface{}
	Sendstatus *chan string
	TaskId     int //该插件所属的taskid
}

type GroupData struct {
	GroupType string
	GroupUrls []interface{}
	Spider    *brohttp.Spider
	Pctx      *context.Context
	Pcancel   *context.CancelFunc
}

func (p *Plugin) Init() {
	p.Pool, _ = ants.NewPoolWithFunc(p.MaxPoolCount, func(args interface{}) { //新建一个带有同类方法的pool对象
		defer p.threadwg.Done()
		data := args.(GroupData)
		for _, f := range p.Callbacks {
			p.mu.Lock()
			scanresult, err := f(data)
			if err != nil {
				logger.Error(err.Error())
			} else {
				p.ScanResult = append(p.ScanResult, scanresult)
			}
			p.mu.Unlock()
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	p.Ctx = &ctx
	p.Cancel = &cancel
}

type PluginCallback func(args interface{}) (*util.ScanResult, error)

func (p *Plugin) Run(args PluginOption) error {
	defer args.PluginWg.Done()
	defer p.Pool.Release()
	var err error
	for k, v := range args.Data {
		p.threadwg.Add(1)
		go func() {
			data := GroupData{GroupType: k, GroupUrls: v, Spider: p.Spider, Pctx: p.Ctx, Pcancel: p.Cancel}
			err = p.Pool.Invoke(data)
			if err != nil {
				logger.Error(err.Error())
			}
		}()
	}
	p.threadwg.Wait()
	logger.Info("Plugin %s is Finish!", p.PluginName)
	if args.IsSocket {
		Element := make(map[string]interface{})
		jsonElement, err := json.Marshal(Element)
		if err != nil {
			logger.Error(err.Error())
		}
		msgstr := string(jsonElement)
		(*args.Sendstatus) <- msgstr
	}
	util.OutputVulnerable(p.ScanResult)
	return err
}