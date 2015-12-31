package index

import (
	"appengine"

	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"sort"
	"time"

	"firego"
)

type Client struct {
	ReqId   string `json:"reqId"`
	ReqTime int64  `json:"reqTime"`
	Comment string `json:"comment"`
}

type ClientArray []Client

func (s ClientArray) Len() int {
	return len(s)
}
func (s ClientArray) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ClientArray) Less(i, j int) bool {
	return s[i].ReqTime >= s[j].ReqTime
}

func init() {
	http.HandleFunc("/insert", handleInsert)
	http.HandleFunc("/remove", handleRemove)
	http.HandleFunc("/delete", handleDelete)
	http.HandleFunc("/list", handleList)
	http.HandleFunc("/table", handleTable)
}

func status(w http.ResponseWriter, reqId string, status int) {
	if reqId == "" {
		reqId = "not provided"
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":%d, "reqId" : "%s"}`, status, reqId)
}

func statusNoMessage(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":%d}`, status)
}

func response(w http.ResponseWriter, reqId string, clients []Client) {
	if reqId == "" {
		reqId = "not provided"
	}
	json, _ := json.Marshal(clients)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":200, "reqId" : "%s", "result" : %s}`, reqId, string(json))
}

func loadClients(r *http.Request, ch chan ClientArray) {
	f := firego.NewGAE(appengine.NewContext(r), DB)
	f.Auth(AUTH)
	var values map[string]Client
	if e := f.Value(&values); e == nil {
		list := make(ClientArray, 0)
		for _, v := range values {
			list = append(list, v)
		}
		sort.Sort(list)
		ch <- list
	} else {
		ch <- nil
	}
}

func showList(w http.ResponseWriter, r *http.Request, reqId string) {
	ch := make(chan ClientArray)
	go loadClients(r, ch)
	list := <-ch
	if list != nil {
		response(w, reqId, list)
	} else {
		status(w, reqId, 500)
	}
}

func handleInsert(w http.ResponseWriter, r *http.Request) {
	client := Client{}
	if bys, e := ioutil.ReadAll(r.Body); e == nil {
		if e := json.Unmarshal(bys, &client); e == nil {
			f := firego.NewGAE(appengine.NewContext(r), DB)
			f.Auth(AUTH)
			childFB := f.Child(client.ReqId)
			if e := childFB.Set(client); e == nil {
				if _, e := f.Push(nil); e == nil { //Like java: fb.child("xxxx").setValue(obj).push()
					status(w, client.ReqId, 200)
				} else {
					s := fmt.Sprintf("%v", e)
					status(w, s, 500)
				}
			} else {
				s := fmt.Sprintf("%v", e)
				status(w, s, 500)
			}
		} else {
			s := fmt.Sprintf("%v", e)
			status(w, s, 500)
		}
	} else {
		s := fmt.Sprintf("%v", e)
		status(w, s, 500)
	}
}

func handleList(w http.ResponseWriter, r *http.Request) {
	client := Client{}
	if bytes, e := ioutil.ReadAll(r.Body); e == nil {
		if string(bytes) == "" {
			showList(w, r, "reqId is empty, it might be sent from desktop.")
		} else {
			if e := json.Unmarshal(bytes, &client); e == nil {
				showList(w, r, client.ReqId)
			} else {
				s := fmt.Sprintf("%v", e)
				status(w, s, 500)
			}
		}
	} else {
		s := fmt.Sprintf("%v", e)
		status(w, s, 500)
	}
}

func handleRemove(w http.ResponseWriter, r *http.Request) {
	if bys, e := ioutil.ReadAll(r.Body); e == nil {
		clientID := string(bys)
		if clientID == "" {
			status(w, "Please give client-ID on post-body.", 500)
		} else {
			var whichItemToRemove string = clientID
			//Delete client which the key(from DB).
			if whichItemToRemove != "" {
				f := firego.NewGAE(appengine.NewContext(r), DB+"/"+whichItemToRemove)
				f.Auth(AUTH)
				if err := f.Remove(); err != nil {
					status(w, fmt.Sprintf("%v", err), 500)
				} else {
					status(w, whichItemToRemove, 200)
				}
			} else {
				s := fmt.Sprintf("%v", e)
				status(w, s, 500)
			}
		}
	} else {
		s := fmt.Sprintf("%v", e)
		status(w, s, 500)
	}
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	client := Client{}
	if bys, e := ioutil.ReadAll(r.Body); e == nil {
		if e := json.Unmarshal(bys, &client); e == nil {
			//Delete client which the key(from DB).
			if client.ReqId != "" {
				f := firego.NewGAE(appengine.NewContext(r), DB+"/"+(client.ReqId))
				f.Auth(AUTH)
				if err := f.Remove(); err != nil {
					status(w, fmt.Sprintf("%v", err), 500)
				} else {
					status(w, client.ReqId, 200)
				}
			} else {
				s := fmt.Sprintf("%v", e)
				status(w, s, 500)
			}
		} else {
			s := fmt.Sprintf("%v", e)
			status(w, s, 500)
		}
	} else {
		s := fmt.Sprintf("%v", e)
		status(w, s, 500)
	}
}

var tableTep = template.Must(template.ParseFiles("table.html"))

func (c Client) RequestTime() (datetime string) {
	tz := "CET"
	location, _ := time.LoadLocation(tz)
	datetime = time.Unix(c.ReqTime/1000, 0).In(location).Format("Mon Jan _2 15:04:05 2006")
	return
}

func showTable(w http.ResponseWriter, r *http.Request, reqId string) {
	ch := make(chan ClientArray)
	go loadClients(r, ch)
	list := <-ch
	if list != nil {
		tableTep.Execute(w, list)
	} else {
		status(w, reqId, 500)
	}
}

func handleTable(w http.ResponseWriter, r *http.Request) {
	client := Client{}
	if bytes, e := ioutil.ReadAll(r.Body); e == nil {
		if string(bytes) == "" {
			showTable(w, r, "reqId is empty, it might be sent from desktop.")
		} else {
			if e := json.Unmarshal(bytes, &client); e == nil {
				showTable(w, r, client.ReqId)
			} else {
				s := fmt.Sprintf("%v", e)
				status(w, s, 500)
			}
		}
	} else {
		s := fmt.Sprintf("%v", e)
		status(w, s, 500)
	}
}
