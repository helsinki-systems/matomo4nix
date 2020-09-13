package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-version"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

var DEBUG bool
var COMMIT_LOG bool
var MATOMO_VERSION *version.Version

const API_BASE = "https://plugins.matomo.org"

type Plugin struct {
	Sha256      string `json:"sha256"`
	Url         string `json:"url"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
	License     string `json:"license"`
	DisplayName string `json:"displayName"`
}

type PluginJson map[string]Plugin

func loadFile(t string) (PluginJson, error) {
	fname := t + ".json"
	log.Print("Loading " + fname)
	file, _ := os.OpenFile(fname, os.O_CREATE|os.O_RDONLY, 0644)
	defer file.Close()
	var m PluginJson
	dat, err := ioutil.ReadAll(file)
	if err != nil {
		log.Printf("Failed to read file %s: %e", fname, err)
	}
	err = json.Unmarshal(dat, &m)
	if err != nil {
		log.Printf("Failed to parse %s: %e", fname, err)
	}
	log.Printf("Loaded %s", fname)
	return m, err
}

func writeLog(t string, po, pn PluginJson) {
	file, _ := os.OpenFile(t+"-new.log", os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	log.Printf("Writing %s-new.log", t)
	for k, np := range pn {
		op, isOld := po[k]
		if !isOld {
			file.WriteString(fmt.Sprintf("ADD %s %s\n", k, np.Version))
		} else if isOld && np.Version != op.Version {
			file.WriteString(fmt.Sprintf("UPD %s %s -> %s\n", k, op.Version, np.Version))
		}
	}
	log.Printf("Replacing %s.log with %s-new.log", t, t)
	os.Rename(t+"-new.log", t+".log")
}

func writeFile(t string, c PluginJson) {
	file, _ := os.OpenFile(t+"-new.json", os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	log.Printf("Writing %s-new.json", t)
	enc.Encode(c)
	log.Printf("Replacing %s.json with %s-new.json", t, t)
	os.Rename(t+"-new.json", t+".json")
}

func prefetch(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Print("Prefetch failed for: ", url, err)
		return "", err
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Prefetch failed reading body for: ", url, err)
		return "", err
	}
	sha256 := fmt.Sprintf("%x", sha256.Sum256(contents))

	return sha256, err
}

// copy every element from every map into the resulting map
// meaning, merge all maps with the later maps having precedence over the previous one(s)
func mergePs(ps ...PluginJson) PluginJson {
	res := make(PluginJson)
	for _, m := range ps {
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}

type ApiPluginVersion struct {
	Name     string            `json:"name"` // I'd call this version, but ok
	Download string            `json:"download"`
	License  map[string]string `json:"license"`
	Requires interface{}       `json:"requires"`
}

type ApiPlugin struct {
	Name           string             `json:"name"`
	DisplayName    string             `json:"displayName"`
	Versions       []ApiPluginVersion `json:"versions"`
	IsDownloadable bool               `json:"isDownloadable"`
	Description    string             `json:"description"`
	Homepage       string             `json:"homepage"`
	IsTheme        bool               `json:"isTheme"`
}

type ApiResponse struct {
	Plugins []ApiPlugin `json:"plugins"`
}

func queryApi(t string) (ApiResponse, error) {
	url := API_BASE + "/api/2.0/" + t
	log.Printf("Querying API at %s", url)
	resp, err := http.Get(url)
	var apiResponse ApiResponse
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Print("API query failed (", resp.Status, ") for: ", url, err)
		return apiResponse, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("API query failed to read body: ", url, err)
		return apiResponse, err
	}
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		log.Print("API query failed to parse JSON: ", body, err)
		return apiResponse, err
	}
	return apiResponse, nil
}

func update(t string) {
	log.Printf("Starting to process %s", t)
	reg, _ := regexp.Compile("[^0-9.]+")
	j, err := queryApi(t)
	if err != nil {
		panic(err)
	}
	po, err := loadFile(t)
	if err != nil {
		panic(err)
	}
	pn := make(PluginJson)
	for _, p := range j.Plugins {
		if !p.IsDownloadable {
			continue
		}
		var np Plugin
		np.Description = p.Description
		np.Homepage = p.Homepage
		np.DisplayName = p.DisplayName
		for _, ver := range p.Versions {
			compatible := false
			// in case you're wondering wtf this is about, why the []interface{}, why check the length?
			// the answser is php. FUCK php. it randomly returns EITHER "[]" or "{}" if there are no requirements.
			// I wish I were making this shit up…
			switch reqs := ver.Requires.(type) {
			case []interface{}:
				compatible = true
			case map[string]interface{}:
				if len(reqs) == 0 {
					compatible = true
				} else {
					for reqKey, reqVal := range reqs {
						if reqKey == "piwik" || reqKey == "matomo" {
							split := strings.Split(reqVal.(string), ",")
							// log.Print("split rv: ", split)
							leq := -1
							geq := -1
							for _, s := range split {
								trimmed := reg.ReplaceAllString(s, "")
								reqVer, err := version.NewVersion(trimmed)
								if err != nil {
									log.Printf("Error parsing version in requirement: %e", err)
									continue
								}
								if strings.HasPrefix(s, ">") {
									if MATOMO_VERSION.GreaterThan(reqVer) {
										geq = 1
									} else {
										geq = 0
									}
								} else if strings.HasPrefix(s, ">=") {
									if MATOMO_VERSION.GreaterThanOrEqual(reqVer) {
										geq = 1
									} else {
										geq = 0
									}
								} else if strings.HasPrefix(s, "<=") {
									if MATOMO_VERSION.LessThanOrEqual(reqVer) {
										leq = 1
									} else {
										leq = 0
									}
								} else if strings.HasPrefix(s, "<") {
									if MATOMO_VERSION.LessThan(reqVer) {
										leq = 1
									} else {
										leq = 0
									}
								}
							}

							compatible = (leq == 1 && geq == 1) || (leq == -1 && geq == 1)
						}
					}
				}
			default:
				panic("what?")
			}
			if compatible {
				np.Url = API_BASE + ver.Download
				np.Version = ver.Name
				np.License = ver.License["name"]
				pn[p.Name] = np
			}
		}
		log.Printf("Found plugin %s (%s) at %s", p.Name, np.Version, np.Url)
	}

	for k, _ := range pn {
		needsPrefetch := false
		op, isOld := po[k]
		np, _ := pn[k]

		if !isOld {
			log.Printf("New plugin found -> prefetching %s (%s) from %s", k, np.Version, np.Url)
			needsPrefetch = true
		} else {
			if np.Version != op.Version {
				needsPrefetch = true
				log.Printf("Plugin was updated -> prefetching %s (%s) from %s", k, np.Version, np.Url)
			}
		}

		if needsPrefetch {
			sha256, err := prefetch(np.Url)
			if err != nil {
				continue
			}
			np.Sha256 = sha256
		} else {
			np.Sha256 = op.Sha256
		}
		pn[k] = np
	}

	writeFile(t, mergePs(po, pn))
	writeLog(t, po, pn)
	log.Printf("Finished processing %s", t)
}

func main() {
	_, DEBUG = os.LookupEnv("DEBUG")
	_, COMMIT_LOG = os.LookupEnv("COMMIT_LOG")
	var isSet bool
	MATOMO_VERSION_ENV, isSet := os.LookupEnv("MATOMO_VERSION")
	if !isSet {
		log.Fatal("MATOMO_VERSION needs to be set to the matomo release, so compatibility can be checked.")
		os.Exit(1)
	}
	var err error
	MATOMO_VERSION, err = version.NewVersion(MATOMO_VERSION_ENV)
	if err != nil {
		log.Fatal("Error parsing MATOMO_VERSION.")
		panic(err)
	}

	update("themes")
	update("plugins")
}
