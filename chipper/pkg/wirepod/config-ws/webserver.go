package webserver

import (
	"bufio"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/kercre123/chipper/pkg/logger"
)

type intentsStruct []struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Utterances  []string `json:"utterances"`
	Intent      string   `json:"intent"`
	Params      struct {
		ParamName  string `json:"paramname"`
		ParamValue string `json:"paramvalue"`
	} `json:"params"`
	Exec           string   `json:"exec"`
	ExecArgs       []string `json:"execargs"`
	IsSystemIntent bool     `json:"issystem"`
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	switch {
	default:
		http.Error(w, "not found", http.StatusNotFound)
		return
	case r.URL.Path == "/api/add_custom_intent":
		name := r.FormValue("name")
		description := r.FormValue("description")
		utterances := r.FormValue("utterances")
		intent := r.FormValue("intent")
		paramName := r.FormValue("paramname")
		paramValue := r.FormValue("paramvalue")
		exec := r.FormValue("exec")
		execArgs := r.FormValue("execargs")
		if name == "" || description == "" || utterances == "" || intent == "" {
			fmt.Fprintf(w, "missing required field (name, description, utterances, and intent are required)")
			return
		}
		if _, err := os.Stat("./customIntents.json"); err == nil {
			logger.Println("Found customIntents.json")
			var customIntentJSON intentsStruct
			customIntentJSONFile, _ := os.ReadFile("./customIntents.json")
			json.Unmarshal(customIntentJSONFile, &customIntentJSON)
			logger.Println("Number of custom intents (current): " + strconv.Itoa(len(customIntentJSON)))
			customIntentJSON = append(customIntentJSON, struct {
				Name        string   `json:"name"`
				Description string   `json:"description"`
				Utterances  []string `json:"utterances"`
				Intent      string   `json:"intent"`
				Params      struct {
					ParamName  string `json:"paramname"`
					ParamValue string `json:"paramvalue"`
				} `json:"params"`
				Exec           string   `json:"exec"`
				ExecArgs       []string `json:"execargs"`
				IsSystemIntent bool     `json:"issystem"`
			}{Name: name, Description: description, Utterances: strings.Split(utterances, ","), Intent: intent, Params: struct {
				ParamName  string `json:"paramname"`
				ParamValue string `json:"paramvalue"`
			}{ParamName: paramName, ParamValue: paramValue}, Exec: exec, ExecArgs: strings.Split(execArgs, ","), IsSystemIntent: false})
			customIntentJSONFile, _ = json.Marshal(customIntentJSON)
			os.WriteFile("./customIntents.json", customIntentJSONFile, 0644)
		} else {
			logger.Println("Creating customIntents.json")
			customIntentJSONFile, _ := json.Marshal([]struct {
				Name        string   `json:"name"`
				Description string   `json:"description"`
				Utterances  []string `json:"utterances"`
				Intent      string   `json:"intent"`
				Params      struct {
					ParamName  string `json:"paramname"`
					ParamValue string `json:"paramvalue"`
				} `json:"params"`
				Exec           string   `json:"exec"`
				ExecArgs       []string `json:"execargs"`
				IsSystemIntent bool     `json:"issystem"`
			}{{Name: name, Description: description, Utterances: strings.Split(utterances, ","), Intent: intent, Params: struct {
				ParamName  string `json:"paramname"`
				ParamValue string `json:"paramvalue"`
			}{ParamName: paramName, ParamValue: paramValue}, Exec: exec, ExecArgs: strings.Split(execArgs, ","), IsSystemIntent: false}})
			os.WriteFile("./customIntents.json", customIntentJSONFile, 0644)
		}
		fmt.Fprintf(w, "intent added successfully")
		return
	case r.URL.Path == "/api/edit_custom_intent":
		number := r.FormValue("number")
		name := r.FormValue("name")
		description := r.FormValue("description")
		utterances := r.FormValue("utterances")
		intent := r.FormValue("intent")
		paramName := r.FormValue("paramname")
		paramValue := r.FormValue("paramvalue")
		exec := r.FormValue("exec")
		execArgs := r.FormValue("execargs")
		if number == "" {
			fmt.Fprintf(w, "err: a number is required")
			return
		}
		if name == "" && description == "" && utterances == "" && intent == "" && paramName == "" && paramValue == "" && exec == "" {
			fmt.Fprintf(w, "err: an entry must be edited")
			return
		}
		if _, err := os.Stat("./customIntents.json"); err == nil {
			// do nothing
		} else {
			fmt.Fprintf(w, "err: you must create an intent first")
			return
		}
		var customIntentJSON intentsStruct
		customIntentJSONFile, err := os.ReadFile("./customIntents.json")
		if err != nil {
			logger.Println(err)
		}
		json.Unmarshal(customIntentJSONFile, &customIntentJSON)
		newNumbera, _ := strconv.Atoi(number)
		newNumber := newNumbera - 1
		if newNumber > len(customIntentJSON) {
			fmt.Fprintf(w, "err: there are only "+strconv.Itoa(len(customIntentJSON))+" intents")
			return
		}
		if name != "" {
			customIntentJSON[newNumber].Name = name
		}
		if description != "" {
			customIntentJSON[newNumber].Description = description
		}
		if utterances != "" {
			customIntentJSON[newNumber].Utterances = strings.Split(utterances, ",")
		}
		if intent != "" {
			customIntentJSON[newNumber].Intent = intent
		}
		if paramName != "" {
			customIntentJSON[newNumber].Params.ParamName = paramName
		}
		if paramValue != "" {
			customIntentJSON[newNumber].Params.ParamValue = paramValue
		}
		if exec != "" {
			customIntentJSON[newNumber].Exec = exec
		}
		if execArgs != "" {
			customIntentJSON[newNumber].ExecArgs = strings.Split(execArgs, ",")
		}
		customIntentJSON[newNumber].IsSystemIntent = false
		newCustomIntentJSONFile, _ := json.Marshal(customIntentJSON)
		os.WriteFile("./customIntents.json", newCustomIntentJSONFile, 0644)
		fmt.Fprintf(w, "intent edited successfully")
		return
	case r.URL.Path == "/api/get_custom_intents_json":
		if _, err := os.Stat("./customIntents.json"); err == nil {
			// do nothing
		} else {
			fmt.Fprintf(w, "err: you must create an intent first")
			return
		}
		customIntentJSONFile, err := os.ReadFile("./customIntents.json")
		if err != nil {
			logger.Println(err)
		}
		fmt.Fprint(w, string(customIntentJSONFile))
		return
	case r.URL.Path == "/api/remove_custom_intent":
		number := r.FormValue("number")
		if number == "" {
			fmt.Fprintf(w, "err: a number is required")
			return
		}
		if _, err := os.Stat("./customIntents.json"); err == nil {
			// do nothing
		} else {
			fmt.Fprintf(w, "err: you must create an intent first")
			return
		}
		var customIntentJSON intentsStruct
		customIntentJSONFile, err := os.ReadFile("./customIntents.json")
		if err != nil {
			logger.Println(err)
		}
		json.Unmarshal(customIntentJSONFile, &customIntentJSON)
		newNumbera, _ := strconv.Atoi(number)
		newNumber := newNumbera - 1
		if newNumber > len(customIntentJSON) {
			fmt.Fprintf(w, "err: there are only "+strconv.Itoa(len(customIntentJSON))+" intents")
			return
		}
		customIntentJSON = append(customIntentJSON[:newNumber], customIntentJSON[newNumber+1:]...)
		newCustomIntentJSONFile, _ := json.Marshal(customIntentJSON)
		os.WriteFile("./customIntents.json", newCustomIntentJSONFile, 0644)
		fmt.Fprintf(w, "intent removed successfully")
		return
	case r.URL.Path == "/api/add_bot":
		botESN := r.FormValue("esn")
		botLocation := r.FormValue("location")
		botUnits := r.FormValue("units")
		botFirmwarePrefix := r.FormValue("firmwareprefix")
		var is_early_opus bool
		var use_play_specific bool
		if botESN == "" || botLocation == "" || botUnits == "" || botFirmwarePrefix == "" {
			fmt.Fprintf(w, "err: all fields are required")
			return
		}
		firmwareSplit := strings.Split(botFirmwarePrefix, ".")
		if len(firmwareSplit) != 2 {
			fmt.Fprintf(w, "err: firmware prefix must be in the format: 1.5")
			return
		}
		if botUnits != "F" && botUnits != "C" {
			fmt.Fprintf(w, "err: units must be either F or C")
			return
		}
		firmware1, _ := strconv.Atoi(firmwareSplit[0])
		firmware2, err := strconv.Atoi(firmwareSplit[1])
		if err != nil {
			fmt.Fprintf(w, "err: firmware prefix must be in the format: 1.5")
			return
		}
		if firmware1 >= 1 && firmware2 < 6 {
			is_early_opus = false
			use_play_specific = true
		} else if firmware1 >= 1 && firmware2 >= 6 {
			is_early_opus = false
			use_play_specific = false
		} else if firmware1 == 0 {
			is_early_opus = true
			use_play_specific = true
		} else {
			fmt.Fprintf(w, "err: firmware prefix must be in the format: 1.5")
			return
		}
		type botConfigStruct []struct {
			Esn             string `json:"esn"`
			Location        string `json:"location"`
			Units           string `json:"units"`
			UsePlaySpecific bool   `json:"use_play_specific"`
			IsEarlyOpus     bool   `json:"is_early_opus"`
		}
		var botConfig botConfigStruct
		if _, err := os.Stat("./botConfig.json"); err == nil {
			// read botConfig.json and append to it with the form information
			botConfigFile, err := os.ReadFile("./botConfig.json")
			if err != nil {
				logger.Println(err)
			}
			json.Unmarshal(botConfigFile, &botConfig)
			botConfig = append(botConfig, struct {
				Esn             string `json:"esn"`
				Location        string `json:"location"`
				Units           string `json:"units"`
				UsePlaySpecific bool   `json:"use_play_specific"`
				IsEarlyOpus     bool   `json:"is_early_opus"`
			}{Esn: botESN, Location: botLocation, Units: botUnits, UsePlaySpecific: use_play_specific, IsEarlyOpus: is_early_opus})
			newBotConfigJSONFile, _ := json.Marshal(botConfig)
			os.WriteFile("./botConfig.json", newBotConfigJSONFile, 0644)
		} else {
			botConfig = append(botConfig, struct {
				Esn             string `json:"esn"`
				Location        string `json:"location"`
				Units           string `json:"units"`
				UsePlaySpecific bool   `json:"use_play_specific"`
				IsEarlyOpus     bool   `json:"is_early_opus"`
			}{Esn: botESN, Location: botLocation, Units: botUnits, UsePlaySpecific: use_play_specific, IsEarlyOpus: is_early_opus})
			newBotConfigJSONFile, _ := json.Marshal(botConfig)
			os.WriteFile("./botConfig.json", newBotConfigJSONFile, 0644)
		}
		fmt.Fprintf(w, "bot added successfully")
		return
	case r.URL.Path == "/api/remove_bot":
		number := r.FormValue("number")
		if _, err := os.Stat("./botConfig.json"); err == nil {
			// do nothing
		} else {
			fmt.Fprintf(w, "err: you must create a bot first")
			return
		}
		type botConfigStruct []struct {
			Esn             string `json:"esn"`
			Location        string `json:"location"`
			Units           string `json:"units"`
			UsePlaySpecific bool   `json:"use_play_specific"`
			IsEarlyOpus     bool   `json:"is_early_opus"`
		}
		var botConfigJSON botConfigStruct
		botConfigJSONFile, err := os.ReadFile("./botConfig.json")
		if err != nil {
			logger.Println(err)
		}
		json.Unmarshal(botConfigJSONFile, &botConfigJSON)
		newNumbera, _ := strconv.Atoi(number)
		newNumber := newNumbera - 1
		if newNumber > len(botConfigJSON) {
			fmt.Fprintf(w, "err: there are only "+strconv.Itoa(len(botConfigJSON))+" bots")
			return
		}
		logger.Println(botConfigJSON[newNumber].Esn + " bot is being removed")
		botConfigJSON = append(botConfigJSON[:newNumber], botConfigJSON[newNumber+1:]...)
		newBotConfigJSONFile, _ := json.Marshal(botConfigJSON)
		os.WriteFile("./botConfig.json", newBotConfigJSONFile, 0644)
		fmt.Fprintf(w, "bot removed successfully")
		return
	case r.URL.Path == "/api/edit_bot":
		number := r.FormValue("number")
		botESN := r.FormValue("esn")
		botLocation := r.FormValue("location")
		botUnits := r.FormValue("units")
		botFirmwarePrefix := r.FormValue("firmwareprefix")
		if botESN == "" || botLocation == "" || botUnits == "" || botFirmwarePrefix == "" {
			fmt.Fprintf(w, "err: all fields are required")
			return
		}
		firmwareSplit := strings.Split(botFirmwarePrefix, ".")
		if len(firmwareSplit) != 2 {
			fmt.Fprintf(w, "err: firmware prefix must be in the format: 1.5")
			return
		}
		if botUnits != "F" && botUnits != "C" {
			fmt.Fprintf(w, "err: units must be either F or C")
			return
		}
		var is_early_opus bool
		var use_play_specific bool
		firmware1, _ := strconv.Atoi(firmwareSplit[0])
		firmware2, err := strconv.Atoi(firmwareSplit[1])
		if err != nil {
			fmt.Fprintf(w, "err: firmware prefix must be in the format: 1.5")
			return
		}
		if firmware1 >= 1 && firmware2 < 6 {
			is_early_opus = false
			use_play_specific = true
		} else if firmware1 >= 1 && firmware2 >= 6 {
			is_early_opus = false
			use_play_specific = false
		} else if firmware1 == 0 {
			is_early_opus = true
			use_play_specific = true
		} else {
			fmt.Fprintf(w, "err: firmware prefix must be in the format: 1.5")
			return
		}
		type botConfigStruct []struct {
			Esn             string `json:"esn"`
			Location        string `json:"location"`
			Units           string `json:"units"`
			UsePlaySpecific bool   `json:"use_play_specific"`
			IsEarlyOpus     bool   `json:"is_early_opus"`
		}
		var botConfig botConfigStruct
		if _, err := os.Stat("./botConfig.json"); err == nil {
			// read botConfig.json and append to it with the form information
			botConfigFile, err := os.ReadFile("./botConfig.json")
			if err != nil {
				logger.Println(err)
			}
			json.Unmarshal(botConfigFile, &botConfig)
			newNumbera, _ := strconv.Atoi(number)
			newNumber := newNumbera - 1
			botConfig[newNumber].Esn = botESN
			botConfig[newNumber].Location = botLocation
			botConfig[newNumber].Units = botUnits
			botConfig[newNumber].UsePlaySpecific = use_play_specific
			botConfig[newNumber].IsEarlyOpus = is_early_opus
			newBotConfigJSONFile, _ := json.Marshal(botConfig)
			os.WriteFile("./botConfig.json", newBotConfigJSONFile, 0644)
		} else {
			fmt.Fprintln(w, "err: you must create a bot first")
			return
		}
		fmt.Fprintf(w, "bot edited successfully")
		return
	case r.URL.Path == "/api/get_bot_json":
		if _, err := os.Stat("./botConfig.json"); err == nil {
			// do nothing
		} else {
			fmt.Fprintf(w, "err: you must add a bot first")
			return
		}
		botConfigJSONFile, err := os.ReadFile("./botConfig.json")
		if err != nil {
			logger.Println(err)
		}
		fmt.Fprint(w, string(botConfigJSONFile))
		return
	case r.URL.Path == "/api/debug":
		resp, err := http.Get("https://session-certs.token.global.anki-services.com/vic/00e20145")
		if err != nil {
			fmt.Println(err)
		}
		certBytes, _ := io.ReadAll(resp.Body)
		block, _ := pem.Decode(certBytes)
		certBytes = block.Bytes
		cert, err := x509.ParseCertificate(certBytes)
		if err != nil {
			fmt.Println(err)
		}
		botName := cert.Issuer.CommonName
		fmt.Println(botName)
		fmt.Fprintf(w, "done")
		return
	case r.URL.Path == "/api/set_weather_api":
		weatherProvider := r.FormValue("provider")
		weatherAPIKey := r.FormValue("api_key")
		// Patch source.sh
		lines, err := readLines("source.sh")
		if err == nil {
			var outlines []string
			for _, line := range lines {
				if strings.HasPrefix(line, "export WEATHERAPI_ENABLED") {
					if weatherProvider == "" {
						line = "export WEATHERAPI_ENABLED=false"
					} else {
						line = "export WEATHERAPI_ENABLED=true"
					}
				} else if strings.HasPrefix(line, "export WEATHERAPI_PROVIDER") {
					line = "export WEATHERAPI_PROVIDER=" + weatherProvider
				} else if strings.HasPrefix(line, "export WEATHERAPI_KEY") {
					line = "export WEATHERAPI_KEY=" + weatherAPIKey
				}
				outlines = append(outlines, line)
			}
			writeLines(outlines, "source.sh")
			fmt.Fprintf(w, "Changes saved. Restart needed.")
		}
		return
	case r.URL.Path == "/api/get_weather_api":
		weatherEnabled := 0
		weatherProvider := ""
		weatherAPIKey := ""
		lines, err := readLines("source.sh")
		if err == nil {
			for _, line := range lines {
				if strings.HasPrefix(line, "export WEATHERAPI_ENABLED=true") {
					weatherEnabled = 1
				} else if strings.HasPrefix(line, "export WEATHERAPI_PROVIDER=") {
					weatherProvider = strings.SplitAfter(line, "export WEATHERAPI_PROVIDER=")[1]
				} else if strings.HasPrefix(line, "export WEATHERAPI_KEY=") {
					weatherAPIKey = strings.SplitAfter(line, "export WEATHERAPI_KEY=")[1]
				}
			}
		}
		fmt.Fprintf(w, "{ ")
		fmt.Fprintf(w, "  \"weatherEnabled\": %d,", weatherEnabled)
		fmt.Fprintf(w, "  \"weatherProvider\": \"%s\",", weatherProvider)
		fmt.Fprintf(w, "  \"weatherApiKey\": \"%s\"", weatherAPIKey)
		fmt.Fprintf(w, "}")
		return
	case r.URL.Path == "/api/set_kg_api":
		kgProvider := r.FormValue("provider")
		kgAPIKey := r.FormValue("api_key")
		// Patch source.sh
		lines, err := readLines("source.sh")
		var outlines []string
		if err == nil {
			for _, line := range lines {
				if strings.HasPrefix(line, "export KNOWLEDGE_ENABLED") {
					if kgProvider == "" {
						line = "export KNOWLEDGE_ENABLED=false"
					} else {
						line = "export KNOWLEDGE_ENABLED=true"
					}
				} else if strings.HasPrefix(line, "export KNOWLEDGE_PROVIDER") {
					line = "export KNOWLEDGE_PROVIDER=" + kgProvider
				} else if strings.HasPrefix(line, "export KNOWLEDGE_KEY") {
					line = "export KNOWLEDGE_KEY=" + kgAPIKey
				}
				outlines = append(outlines, line)
			}
			writeLines(outlines, "source.sh")
			fmt.Fprintf(w, "Changes saved. Restart needed.")
		}
		return
	case r.URL.Path == "/api/get_kg_api":
		kgEnabled := 0
		kgProvider := ""
		kgAPIKey := ""
		lines, err := readLines("source.sh")
		if err == nil {
			for _, line := range lines {
				if strings.HasPrefix(line, "export KNOWLEDGE_ENABLED=true") {
					kgEnabled = 1
				} else if strings.HasPrefix(line, "export KNOWLEDGE_PROVIDER=") {
					kgProvider = strings.SplitAfter(line, "export KNOWLEDGE_PROVIDER=")[1]
				} else if strings.HasPrefix(line, "export KNOWLEDGE_KEY=") {
					kgAPIKey = strings.SplitAfter(line, "export KNOWLEDGE_KEY=")[1]
				}
			}
		}
		fmt.Fprintf(w, "{ ")
		fmt.Fprintf(w, "  \"kgEnabled\": %d,", kgEnabled)
		fmt.Fprintf(w, "  \"kgProvider\": \"%s\",", kgProvider)
		fmt.Fprintf(w, "  \"kgApiKey\": \"%s\"", kgAPIKey)
		fmt.Fprintf(w, "}")
		return
	case r.URL.Path == "/api/set_stt_info":
		language := r.FormValue("language")

		// Patch source.sh
		lines, err := readLines("source.sh")
		var outlines []string
		if err == nil {
			for _, line := range lines {
				if strings.HasPrefix(line, "export STT_LANGUAGE") {
					line = "export STT_LANGUAGE=" + language
				}
				outlines = append(outlines, line)
			}
			writeLines(outlines, "source.sh")
			fmt.Fprintf(w, "Changes saved. Restart needed.")
		}
		return
	case r.URL.Path == "/api/get_stt_info":
		sttLanguage := ""
		sttProvider := ""
		lines, err := readLines("source.sh")
		if err == nil {
			for _, line := range lines {
				if strings.HasPrefix(line, "export STT_SERVICE=") {
					sttProvider = strings.SplitAfter(line, "export STT_SERVICE=")[1]
				} else if strings.HasPrefix(line, "export STT_LANGUAGE=") {
					sttLanguage = strings.SplitAfter(line, "export STT_LANGUAGE=")[1]
				}
			}
		}
		fmt.Fprintf(w, "{ ")
		fmt.Fprintf(w, "  \"sttProvider\": \"%s\",", sttProvider)
		fmt.Fprintf(w, "  \"sttLanguage\": \"%s\"", sttLanguage)
		fmt.Fprintf(w, "}")
		return
	case r.URL.Path == "/api/reset":
		cmd := exec.Command("/bin/sh", "-c", "sudo systemctl restart wire-pod")
		err := cmd.Run()
		if err != nil {
			fmt.Fprintf(w, "%s", err.Error())
			log.Fatal(err)
		}
		return
	}
}

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// writeLines writes the lines to the given file.
func writeLines(lines []string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	return w.Flush()
}

func StartWebServer() {
	var webPort string
	http.HandleFunc("/api/", apiHandler)
	fileServer := http.FileServer(http.Dir("./webroot"))
	http.Handle("/", fileServer)
	if os.Getenv("WEBSERVER_PORT") != "" {
		if _, err := strconv.Atoi(os.Getenv("WEBSERVER_PORT")); err == nil {
			webPort = os.Getenv("WEBSERVER_PORT")
		} else {
			logger.Println("WEBSERVER_PORT contains letters, using default of 8080")
			webPort = "8080"
		}
	} else {
		webPort = "8080"
	}
	fmt.Printf("Starting webserver at port " + webPort + " (http://localhost:" + webPort + ")\n")
	if err := http.ListenAndServe(":"+webPort, nil); err != nil {
		log.Fatal(err)
	}
}