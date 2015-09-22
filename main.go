package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"code.google.com/p/gcfg"
	r "github.com/dancannon/gorethink"

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
	//if (req.param("hub.verify_token") == config.instagram.verify)
	//res.send(req.param("hub.challenge"));
	//else res.status(500).json({err: "Verify token incorrect"});

	//w.Header().Set("Content-Type", "application/json")

	//result := "tmp"
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

	//var params = {
	//client_id: config.instagram.client,
	//client_secret: config.instagram.secret,
	//verify_token: config.instagram.verify,
	//object: "tag", aspect: "media", object_id: tagName,
	//callback_url: "http://" + config.host + "/publish/photo"
	//};

	//request.post({url: api + "subscriptions", form: params},
	//function(err, response, body) {
	//if (err) console.log("Failed to subscribe:", err)
	//else console.log("Subscribed to tag:", tagName);
	//});

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
		fmt.Println("string: ")
		fmt.Println(string(bodyContent[1 : len(bodyContent)-1]))
		err = json.Unmarshal(bodyContent[1:len(bodyContent)-1], &data)
	}
	if err != nil {
		fmt.Println("Unable to unmarshall the JSON request", err)
		return
	}
	time := data["time"].(float64)
	fmt.Println(time)
	if time-lastUpdate < 1 {
		fmt.Println("time too close, return!")
		return
	}
	lastUpdate = time

	var path = "https://api.instagram.com/v1/tags/" + cfg.Instagram.TagName + "/media/recent?client_id=" + cfg.Instagram.ClientID
	fmt.Println(path)
	// post data:
	//"changed_aspect": "media",
	//"object": "tag",
	//"object_id": "catsofinstagram",
	//"time": 1414995025,
	//"subscription_id": 14185203,
	//"data": {}

	//r.Table("instacat").Insert()
	res, err := r.HTTP(path).Run(session)
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println(res)

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

	var value interface{}
	cur, err := r.Table("instacat").Changes().Run(session)
	if err != nil {
		log.Println(err.Error())
	}
	for cur.Next(&value) {
		fmt.Println(value)
	}
	//ws := c.Socket()
	//if err = websocket.JSON.Send(ws, data); err != nil {
	//ws.Close()

	//r.table("instacat").changes().run(conn)
	//.then(function(cursor) {
	//cursor.each(function(err, item) {
	//if (item && item.new_val)
	//io.sockets.emit("cat", item.new_val);
	//});
	//})
}

func main() {

	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(cors.Default().Handler)

	// Serve index file
	e.Index("public/index.html")

	//e.ServeFile("/ClientTracker", "./public/ClientTracker.js")

	//apiv2.Get("", APIHandler)
	e.Get("/publish/photo", CallbackHandler)
	e.Post("/publish/photo", ReceiveHandler)
	//apiv2.Get("/overview", OverviewHandler)

	//e.WebSocket("/ws", WebSocketHandler)

	// Starting HTTPS server (Production)
	port := ":3000"
	log.Printf("Starting HTTP service on %s ...", port)
	//subscribeTag()
	e.Run(port)
}
