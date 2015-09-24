package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"code.google.com/p/gcfg"
	r "github.com/dancannon/gorethink"
	"github.com/googollee/go-socket.io"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/parnurzeal/gorequest"
	"github.com/rs/cors"
)

var (
	Version    = "0.1"
	cfg        cfgObject //at types.go
	configFile = flag.String("c", "config.gcfg", "config filename")
	api        = "https://api.instagram.com/v1/"
	lastUpdate float64
	session    *r.Session
	server     *socketio.Server

	//CLIENT_ID     = "7839c51c2a324f46a51c77c91711c8c3"
	//CLIENT_SECRET = "9fbfea5eab08476a88c56f825175501e"
	//tagName       = "catsofinstagram"
)

//
func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("In CallbackHandler!")
	fmt.Println(r.URL.Query().Get("hub.verify_token"))
	if r.URL.Query().Get("hub.verify_token") == cfg.Instagram.Verify {
		fmt.Println("verify hihi OK!!")
		w.Write([]byte(r.URL.Query().Get("hub.challenge")))
	} else {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func subscribeTag() {
	request := gorequest.New()
	url := api + "subscriptions"

	m := map[string]interface{}{
		"client_id":     cfg.Instagram.ClientID,
		"client_secret": cfg.Instagram.ClientSecret,
		"verify_token":  cfg.Instagram.Verify,
		"object_id":     cfg.Instagram.TagName,
		"callback_url":  cfg.Instagram.CallbackUrl + "/publish/photo",
		"object":        "tag",
		"aspect":        "media",
	}

	mJson, _ := json.Marshal(m)
	//fmt.Println(string(mJson))
	//contentReader := bytes.NewReader(mJson)
	//req, _ := http.NewRequest("POST", url, contentReader)
	////req.Header.Set("Content-Type", "application/json")
	//client := &http.Client{}
	//resp, _ := client.Do(req)
	//fmt.Println(resp)

	resp, body, errs := request.Post(url).
		//Send(`{"client_id":"7839c51c2a324f46a51c77c91711c8c3","client_secret":"9fbfea5eab08476a88c56f825175501e","verify_token":"hihi","object_id":"catsofinstagram","callback_url":"http://b099b464.ngrok.io/publish/photo","object":"tag","aspect":"media"}`).End()
		Set("Content-Type", "application/json").
		Set("Accept", "application/json").
		Send(string(mJson)).End()

	//Set("Header", "application/x-www-form-urlencoded").

	if errs != nil {
		log.Println(errs)
		fmt.Println(resp.StatusCode)
		fmt.Println(body)
	} else {
		log.Println("Sucessfully subscribe.")
		fmt.Println(resp.StatusCode)
		fmt.Println(body)
	}
	defer resp.Body.Close()
}

func ReceiveHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Println("In ReceiveHandler!")
	//type ReqBody struct {
	//changed_aspect  string      `json:"changed_aspect"`
	//object          string      `json:"object"`
	//object_id       string      `json:"object_id"`
	//time            int64       `json:"time"`
	//subscription_id int64       `json"subscription_id"`
	//data            interface{} `json:"data"`
	//}
	//var rb ReqBody

	//FIXME bodycontent is array@@, so need to trim first and list quot
	var err error
	data := map[string]interface{}{}

	bodyContent, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Println(err)
	}

	if bodyContent != nil {
		//fmt.Println("string: ")
		//fmt.Println(string(bodyContent[1 : len(bodyContent)-1]))
		err = json.Unmarshal(bodyContent[1:len(bodyContent)-1], &data)
	}
	if err != nil {
		log.Println("Unable to unmarshall the JSON request", err)
		return
	}
	updatetime := data["time"].(float64)
	if updatetime-lastUpdate < 1 {
		log.Println("time too close, return!")
		return
	}
	lastUpdate = updatetime

	var path = "https://api.instagram.com/v1/tags/" + cfg.Instagram.TagName + "/media/recent?client_id=" + cfg.Instagram.ClientID
	// post data:
	//"changed_aspect": "media",
	//"object": "tag",
	//"object_id": "catsofinstagram",
	//"time": 1414995025,
	//"subscription_id": 14185203,
	//"data": {}
	now := time.Now().Format("2006-01-02 15:04:05")

	//res, err := r.Table("instacat").Insert(r.HTTP(path).Field("data").Merge(map[string]interface{}{"time": now}, map[string]interface{}{"place": r.Point(20, 80)})).Run(session)
	res, err := r.Table("instacat").Insert(r.HTTP(path).Field("data").Merge(
		map[string]interface{}{"time": now},
		map[string]interface{}{"place": r.Point(
			r.Row.Field("location").Field("longitude").Default(120.58),
			r.Row.Field("location").Field("latitude").Default(23.58),
		)})).Run(session)

	if err != nil {
		log.Println("Unable to unmarshall the JSON request", err)
		return
	}
	defer res.Close()

}

func init() {

	flag.Parse()
	err := gcfg.ReadFileInto(&cfg, *configFile)
	if err != nil {
		log.Fatal(err)
	}

	session, err = r.Connect(r.ConnectOpts{
		Address:  cfg.Database.Host + ":" + cfg.Database.Port, //localhost:28015
		Database: cfg.Database.DB,                             //DB: cats
	})
	if err != nil {
		log.Fatal("Could not connect")
	}
	res, err := r.DBCreate(cfg.Database.DB).RunWrite(session)
	if err != nil {
		log.Println(err.Error())
	}

	fmt.Printf("%d DB created\n", res.DBsCreated)

	r.DB(cfg.Database.DB).TableCreate("instacat").Run(session)
	log.Println("Create table instacat.")

	r.Table("instacat").IndexCreate("time").Run(session)
	log.Println("Create index time.")
	r.Table("instacat").IndexCreate("place", r.IndexCreateOpts{Geo: true}).Run(session)
	log.Println("Create index place.")
}

func main() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(cors.Default().Handler)
	e.Index("public/index.html")
	e.Static("/", "public")
	//e.Favicon("public/favicon.ico")

	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}

	e.Get("/publish/photo", CallbackHandler)
	e.Post("/publish/photo", ReceiveHandler)
	e.Get("/socket.io/", server)

	port := ":3000"
	log.Printf("Starting HTTP service on %s ...", port)
	go func() {
		//subscribeTag()
		var value map[string]interface{}
		cur, err := r.Table("instacat").Changes().Run(session)
		if err != nil {
			log.Println(err.Error())
		}
		for cur.Next(&value) {
			if value["new_val"] != nil {
				//fmt.Println(value["new_val"])
				server.On("connection", func(so socketio.Socket) {
					so.Emit("cat", value["new_val"])
				})
			}
		}
	}()

	server.On("connection", func(so socketio.Socket) {
		result := make([]interface{}, 10)
		//var result interface{}
		cur, err := r.Table("instacat").OrderBy(r.OrderByOpts{
			Index: r.Desc("time"),
		}).Limit(10).Run(session)
		if err != nil {
			log.Println(err.Error())
		}
		cur.All(&result)
		fmt.Println("get result over !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
		//fmt.Println(result)
		so.Emit("recent", result)
		defer cur.Close()
	})
	e.Run(port)
}
