package processreqs

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"encoding/json"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
)

// Request
type Data struct {
    Audio string `json:"audio"`
    Text  string `json:"text"`
    Image string `json:"image"`
}

type Device struct {
    DeviceID string `json:"device_id"`
    UserName string `json:"user_name"`
}

type Payload struct {
    Data   Data   `json:"data"`
    Device Device `json:"device"`
    ConvID string `json:"conv_id"`
}

// Response
type Command struct {
    Intent string `json:"intent"`
    Args   string `json:"args"`
}

type Result struct {
    Text    string  `json:"text"`
    Audio   string  `json:"audio"`
    Command Command `json:"command"`
}

type Response struct {
    MsgID     string  `json:"msg_id"`
    ConvID    string  `json:"conv_id"`
    Result    Result  `json:"result"`
    InnerData *string `json:"inner_data"`
}

func readImage(filePath string) []byte {
	imageFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	return imageFile
}

func plainaiRequest(text string, imageData []byte, audio_data []byte, deviceId string) string {
	url := "https://victorai.miao.cz/conv"
	//url = "http://localhost:8000/conv"
	victorDeviceId := "victor" + deviceId
	// request server
	// imageData := readImage("/home/unbuilt/Downloads/OIP-C_1.jpeg")
	imageDataBase64 := base64.StdEncoding.EncodeToString(imageData)
	audioData := ""
    println("audio_data", audio_data)
    if audio_data != nil {
	    audioData = base64.StdEncoding.EncodeToString(audio_data)
        print("audioData", audioData)
    }

    data := Data{
        Audio: audioData,
        Text:  text,
        Image: imageDataBase64,
    }

    device := Device{
        DeviceID: victorDeviceId,
        UserName: vars.APIConfig.Knowledge.OpenAIPrompt,
    }

    payload := Payload{
        Data:   data,
        Device: device,
        ConvID: deviceId,
    }

	jsonData, err := json.Marshal(payload)
    if err != nil {
		logger.Println(err)
        return ""
    }

    req_plainai, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
		logger.Println(err)
		return ""
    }

	req_plainai.Header.Set("Content-Type", "application/json")
    req_plainai.Header.Set("x-device-id", victorDeviceId)
    req_plainai.Header.Set("x-subscription-key", vars.APIConfig.Knowledge.Key)

    client := &http.Client{}
    response, err := client.Do(req_plainai)
    if err != nil {
		logger.Println(err)
        return ""
    }
    defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logger.Println(err)
		return ""
	}

	return string(body)
}