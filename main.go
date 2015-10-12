package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
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
	VERSION      = "1.0"
	cfg          cfgObject //at types.go
	configFile   = flag.String("c", "config.gcfg", "config filename")
	api          = "https://api.instagram.com/v1/"
	lastUpdate   float64
	session      *r.Session
	server       *socketio.Server
	callback_url string
)

// Callbackhandler accept Instagram GET and check auth
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

func sub() {
	time.Sleep(2 * time.Second)
	//payload := "client_id=7839c51c2a324f46a51c77c91711c8c3&client_secret=9fbfea5eab08476a88c56f825175501e&verify_token=hihi&object_id=catsofinstagram&callback_url=http://6f7f13ef.ngrok.io/publish/photo&object=tag&aspect=media"
	payload :=
		"client_id=" + cfg.Instagram.ClientID +
			"&client_secret=" + cfg.Instagram.ClientSecret +
			"&verify_token=" + cfg.Instagram.Verify +
			"&object_id=" + cfg.Instagram.TagName +
			"&callback_url=" + callback_url + "/publish/photo" +
			"&object=" + "tag" +
			"&aspect=" + "media"

	resp, err := http.Post("https://api.instagram.com/v1/subscriptions",
		"application/x-www-form-urlencoded",
		//strings.NewReader("client_id=7839c51c2a324f46a51c77c91711c8c3&client_secret=9fbfea5eab08476a88c56f825175501e&verify_token=hihi&object_id=catsofinstagram&callback_url=http://6f7f13ef.ngrok.io/publish/photo&object=tag&aspect=media"))
		strings.NewReader(payload))

	if err != nil {
		fmt.Println(err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("==========================")
	fmt.Println(string(body))

}

// Subscribetag subscribe Instagram real time API, when complete auth, Instagram will continu post newest cats photo
func SubscribeTag() {
	time.Sleep(2 * time.Second)
	request := gorequest.New()
	url := api + "subscriptions"

	m := map[string]interface{}{
		"client_id":     cfg.Instagram.ClientID,
		"client_secret": cfg.Instagram.ClientSecret,
		"verify_token":  cfg.Instagram.Verify,
		"object_id":     cfg.Instagram.TagName,
		//"callback_url":  cfg.Instagram.CallbackUrl + "/publish/photo",
		"callback_url": callback_url + "/publish/photo",
		"object":       "tag",
		"aspect":       "media",
	}

	mJson, _ := json.Marshal(m)
	fmt.Println(string(mJson))
	//contentReader := bytes.NewReader(mJson)
	//req, _ := http.NewRequest("POST", url, contentReader)
	////req.Header.Set("Content-Type", "application/json")
	//client := &http.Client{}
	//resp, _ := client.Do(req)
	//fmt.Println(resp)

	resp, body, errs := request.Post(url).
		//Send(`{"client_id":"7839c51c2a324f46a51c77c91711c8c3","client_secret":"9fbfea5eab08476a88c56f825175501e","verify_token":"hihi","object_id":"catsofinstagram","callback_url":"http://b099b464.ngrok.io/publish/photo","object":"tag","aspect":"media"}`).End()
		Set("Header", "application/x-www-form-urlencoded").
		Send(string(mJson)).End()

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

// Receivehandler handle POST. Instagram will continue POST newest cats photo
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

	//FIXME bodycontent is array@@, so need to trim first and list quot
	var err error
	data := map[string]interface{}{}

	bodyContent, err := ioutil.ReadAll(req.Body)
	if err != nil {
		fmt.Println(err)
	}

	if bodyContent != nil {
		err = json.Unmarshal(bodyContent[1:len(bodyContent)-1], &data)
	}
	if err != nil {
		log.Println("Unable to unmarshall the JSON request", err)
		return
	}
	updatetime := data["time"].(float64)
	if updatetime-lastUpdate < 1 {
		log.Println("Time too close, return!\n")
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

	// Some photo has no geographical coordiate, so randomly choose their location for demo use.
	res, err := r.Table("instacat").Insert(r.HTTP(path).Field("data").Merge(
		map[string]interface{}{"time": now},
		map[string]interface{}{"place": r.Point(
			r.Row.Field("location").Field("longitude").Default(120+rand.Intn(10)),
			r.Row.Field("location").Field("latitude").Default(23+rand.Intn(10)),
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

// RealtimeChange use RethinkDB's changefeed method to continuely polling newly added photo, and emit to the socket.
func RealtimeChangefeed(so socketio.Socket) {
	//SubscribeTag()
	var value map[string]interface{}
	cur, err := r.Table("instacat").Changes().Run(session)
	if err != nil {
		log.Println(err.Error())
	}
	for cur.Next(&value) {
		if value["new_val"] != nil {
			log.Println("New Cat Come!!!!")
			so.Emit("cat", value["new_val"])
		}
	}
}

func main() {
	log.Println("VERSION: ", VERSION)
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(cors.Default().Handler)
	e.Index("public/index.html")
	e.Static("/", "public")

	if len(os.Args) < 2 {
		fmt.Println("\nPlease input callback url.\n  Usage: app.exe [callback_url]")
		os.Exit(0)
	}
	callback_url = os.Args[1]
	//e.Favicon("public/favicon.ico")

	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	e.Get("/publish/photo", CallbackHandler)
	e.Post("/publish/photo", ReceiveHandler)
	e.Get("/socket.io/", server)

	server.On("connection", func(so socketio.Socket) {
		log.Println("====================== on connection ======================")
		result := make([]interface{}, 12)
		cur, err := r.Table("instacat").OrderBy(r.OrderByOpts{
			Index: r.Desc("time"),
		}).Limit(12).Run(session)
		if err != nil {
			log.Println(err.Error())
		}
		cur.All(&result)
		fmt.Println("Get result over. ")
		err = so.Emit("recent", result)
		if err != nil {
			log.Println(err.Error())
		}
		so.On("disconnect", func() {
			log.Println("on disconnect")
		})

		RealtimeChangefeed(so)
	})

	//go SubscribeTag()
	go sub()
	port := ":3000"
	log.Printf("Starting HTTP service on %s ...", port)
	e.Run(port)

}
