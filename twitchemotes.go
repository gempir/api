package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

func getData(url, key string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// ensures websites return pages in english (e.g. twitter would return french preview
	// when the request came from a french IP.)
	req.Header.Add("Accept-Language", "en-US, en;q=0.9, *;q=0.5")
	req.Header.Set("User-Agent", "chatterino-api-cache/1.0 emote-sets-resolver")

	resp, err := httpClient.Do(req)
	log.Printf("Fetching %s live...", url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Println("Fetched!")
	return body, nil
}

type EmoteSet struct {
	ChannelName string `json:"channel_name"`
	ChannelID   string `json:"channel_id"`
	Type        string `json:"type"`
	Custom      bool   `json:"custom"`
}

var emoteSets map[string]*EmoteSet
var emoteSetMutex sync.Mutex

func addEmoteSet(emoteSetID, channelName, channelID, setType string) {
	emoteSets[emoteSetID] = &EmoteSet{
		ChannelName: channelName,
		ChannelID:   channelID,
		Type:        setType,
		Custom:      true,
	}
}

func refreshEmoteSetCache() {
	if firstRun {
		emoteSetMutex.Lock()
		defer emoteSetMutex.Unlock()
	}

	data, err := getData("https://twitchemotes.com/api_cache/v3/sets.json", "twitchemotes:sets")
	if err != nil {
		panic(err)
	}

	if !firstRun {
		emoteSetMutex.Lock()
		defer emoteSetMutex.Unlock()
	}

	firstRun = false
	emoteSets = make(map[string]*EmoteSet)

	err = json.Unmarshal(data, &emoteSets)
	if err != nil {
		panic(err)
	}

	for k := range emoteSets {
		emoteSets[k].Type = "sub"
	}

	addEmoteSet("13985", "evohistorical2015", "129284508", "sub")

	log.Println("Refreshed emote sets")

	time.AfterFunc(30*time.Minute, refreshEmoteSetCache)
}

func setHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	setID := vars["setID"]
	emoteSetMutex.Lock()
	defer emoteSetMutex.Unlock()
	data, err := json.Marshal(emoteSets[setID])
	if err != nil {
		panic(err)
	}
	log.Printf("%s: returning data %s\n", setID, data)
	_, err = w.Write(data)
	if err != nil {
		panic(err)
	}
}

func handleTwitchEmotes(router *mux.Router) {
	router.HandleFunc("/twitchemotes/set/{setID}/", setHandler).Methods("GET")
}