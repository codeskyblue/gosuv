/*
   分布式实现
   话外：作者的整体代码结构对于我的分布式修改太合适了
*/
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bluele/gcache"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/qiniu/log"
)

type Cluster struct {
	slaves gcache.Cache
	client *http.Client
	suv    *Supervisor
}

func (cluster *Cluster) join() {
	data := url.Values{"slave": []string{cfg.Server.Addr}}
	request, err := http.NewRequest(http.MethodPost, "http://"+cfg.Server.Master+"/distributed/join", strings.NewReader(data.Encode()))
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	if err != nil {
		log.Errorf("join cluster %s : %s", cfg.Server.Master, err)
		return
	}
	cluster.auth(request)
	resp, err := cluster.client.Do(request)
	if err != nil {
		log.Errorf("join cluster %s : %s", cfg.Server.Master, err)
		return
	}
	if resp.StatusCode == http.StatusOK {
		log.Debugf("join to master %s", cfg.Server.Master)
	} else {
		log.Debugf("join to master %s error: %d", cfg.Server.Master, resp.StatusCode)
	}
}

func (cluster *Cluster) auth(request *http.Request) {
	if cfg.Server.HttpAuth.Enabled {
		request.SetBasicAuth(cfg.Server.HttpAuth.User, cfg.Server.HttpAuth.Password)
	}
}

func (cluster *Cluster) dialWebSocket(wsUrl string) (*websocket.Conn, *http.Response, error) {
	var dialer *websocket.Dialer
	if cfg.Server.HttpAuth.Enabled {
		dialer = &websocket.Dialer{Proxy: func(r *http.Request) (*url.URL, error) {
			cluster.auth(r)
			return websocket.DefaultDialer.Proxy(r)
		}}
	} else {
		dialer = websocket.DefaultDialer
	}
	return dialer.Dial(wsUrl, nil)
}

func (cluster *Cluster) requestSlave(url, method string, bodyBuffer *bytes.Buffer) ([]byte, error) {
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	cluster.auth(request)
	resp, err := cluster.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (cluster *Cluster) cmdJoinCluster(w http.ResponseWriter, r *http.Request) {
	slave := r.PostFormValue("slave")
	if slave == "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if strings.HasPrefix(slave, ":") {
		idx := strings.LastIndex(r.RemoteAddr, ":")
		slave = r.RemoteAddr[:idx] + slave
	}
	log.Debugf("%s join cluster.", slave)
	if out, err := cluster.slaves.Get(slave); err != nil || out == nil {
		cluster.suv.broadcastEvent("new slave : " + slave)
	}
	cluster.slaves.Set(slave, slave)
	w.WriteHeader(http.StatusOK)
}

//获取分布式系统下所有的内容
func (cluster *Cluster) cmdQueryDistributedPrograms(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")
	slaves := []string{}
	for _, v := range cluster.slaves.GetALL() {
		if slave, ok := v.(string); ok {
			slaves = append(slaves, slave)
		}
	}
	sort.Strings(slaves)
	jsonOut := "{"
	idx := 0
	for _, slave := range slaves {
		reqUrl := fmt.Sprintf("http://%s/api/programs", slave)
		if body, err := cluster.requestSlave(reqUrl, http.MethodGet, nil); err == nil {
			jsonOut += fmt.Sprintf("\"%s\":%s", slave, body)
		}
		if idx < cluster.slaves.Len()-1 {
			jsonOut += ","
		}
		idx += 1
	}
	jsonOut += "}"
	w.Write([]byte(jsonOut))
}

func (cluster *Cluster) cmdSetting(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	slave := mux.Vars(r)["slave"]
	cluster.suv.renderHTML(w, "setting", map[string]string{
		"Name":  name,
		"Slave": slave,
	})
}

func (cluster *Cluster) cmdWebSocketProxy(w http.ResponseWriter, r *http.Request) {
	sock, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("upgrade:", err)
		return
	}
	defer sock.Close()

	slave := mux.Vars(r)["slave"]
	slaveUri := strings.Replace(r.RequestURI, "/distributed/"+slave, "", 1)
	wsUrl := fmt.Sprintf("ws://%s%s", slave, slaveUri)
	log.Infof("proxy websocket :%s", wsUrl)

	ws, _, err := cluster.dialWebSocket(wsUrl)
	if err != nil {
		log.Error("dial:", err)
		return
	}
	defer ws.Close()

	for {
		messageType, data, err := ws.ReadMessage()
		if err != nil {
			log.Error("read message:", err)
			return
		}
		if messageType == websocket.CloseMessage {
			log.Infof("close socket")
			return
		}
		w, err := sock.NextWriter(messageType)
		if err != nil {
			log.Error("write err:", err)
			return
		}
		_, err = w.Write(data)
		if err != nil {
			log.Error("read:", err)
			return
		}
	}
}

func (cluster *Cluster) slaveHttpProxy(w http.ResponseWriter, r *http.Request) {
	slave := mux.Vars(r)["slave"]
	slaveUri := strings.Replace(r.RequestURI, "/distributed/"+slave, "", 1)
	requestUrl := fmt.Sprintf("http://%s%s", slave, slaveUri)
	log.Infof("proxy :%s %s", r.Method, requestUrl)

	request, err := http.NewRequest(r.Method, requestUrl, r.Body)
	for k, v := range r.Header {
		request.Header.Set(k, strings.Join(v, ","))
	}
	if err != nil {
		log.Error(err)
	}
	cluster.auth(request)
	resp, err := cluster.client.Do(request)
	if err != nil {
		log.Error(err)
	}
	defer resp.Body.Close()

	if body, err := ioutil.ReadAll(resp.Body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	} else {
		for k, v := range resp.Header {
			w.Header().Set(k, strings.Join(v, ","))
		}
		w.Write(body)
		cluster.suv.broadcastEvent("execute ok : " + slaveUri)
	}
}

func newDistributed(suv *Supervisor, hdlr http.Handler) error {
	cluster.suv = suv

	r := hdlr.(*mux.Router)
	r.HandleFunc("/distributed/join", cluster.cmdJoinCluster).Methods("POST")
	r.HandleFunc("/distributed/api/programs", cluster.cmdQueryDistributedPrograms).Methods("GET")
	r.HandleFunc("/distributed/{slave}/settings/{name}", cluster.cmdSetting)
	for _, path := range []string{
		"/distributed/{slave}/api/programs", "/distributed/{slave}/api/programs/{name}",
		"/distributed/{slave}/api/programs/{name}/start", "/distributed/{slave}/api/programs/{name}/stop",
	} {
		r.HandleFunc(path, cluster.slaveHttpProxy)
	}
	r.HandleFunc("/distributed/{slave}/ws/logs/{name}", cluster.cmdWebSocketProxy)
	r.HandleFunc("/distributed/{slave}/ws/perfs/{name}", cluster.cmdWebSocketProxy)

	if cfg.Server.Master != "" {
		go func() {
			t1 := time.NewTimer(time.Second)
			for {
				select {
				case <-t1.C:
					cluster.join()
					t1.Reset(time.Second)
				}
			}
		}()
	}
	return nil
}

var cluster = Cluster{
	slaves: gcache.New(1000).LRU().Expiration(time.Second * 3).Build(),
	client: new(http.Client),
}
