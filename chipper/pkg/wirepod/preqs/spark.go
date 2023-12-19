package processreqs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/gorilla/websocket"
)

var (
	hostUrlV20   = "wss://spark-api.xf-yun.com/v2.1/chat"
	hostUrlV30   = "wss://spark-api.xf-yun.com/v3.1/chat"
	appid     = ""
	apiSecret = ""
	apiKey    = ""

)

func sparkRequest(transcribedText string) string {
	logger.Println("Making request to Spark...")
	d := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	hostUrl := hostUrlV20
	if vars.APIConfig.Knowledge.RobotName == "api30" {
		hostUrl = hostUrlV30
	}
	//握手并建立websocket 连接
	conn, resp, err := d.Dial(assembleAuthUrl1(hostUrl, vars.APIConfig.Knowledge.Key, vars.APIConfig.Knowledge.Model), nil)
	if err != nil {
		panic(readResp(resp) + err.Error())
		return ""
	} else if resp.StatusCode != 101 {
		panic(readResp(resp) + err.Error())
	}

	go func() {

		data := genParams1(vars.APIConfig.Knowledge.ID, transcribedText)
		conn.WriteJSON(data)

	}()

	var answer = ""
	//获取返回的数据
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("read message error:", err)
			break
		}

		var data map[string]interface{}
		err1 := json.Unmarshal(msg, &data)
		if err1 != nil {
			fmt.Println("Error parsing JSON:", err)
			return ""
		}
		fmt.Println(string(msg))
		//解析数据
		payload := data["payload"].(map[string]interface{})
		choices := payload["choices"].(map[string]interface{})
		header := data["header"].(map[string]interface{})
		code := header["code"].(float64)

		if code != 0 {
			fmt.Println(data["payload"])
			return ""
		}
		status := choices["status"].(float64)
		fmt.Println(status)
		text := choices["text"].([]interface{})
		content := text[0].(map[string]interface{})["content"].(string)
		if status != 2 {
			answer += content
		} else {
			fmt.Println("收到最终结果")
			answer += content
			usage := payload["usage"].(map[string]interface{})
			temp := usage["text"].(map[string]interface{})
			totalTokens := temp["total_tokens"].(float64)
 			fmt.Println("total_tokens:", totalTokens)
			conn.Close()
			break
		}

	}
	//输出返回结果
	fmt.Println(answer)
	apiResponse := answer
	logger.Println("Spark response: " + apiResponse)
	return apiResponse
}

// 生成参数
func genParams1(appid, question string) map[string]interface{} { // 根据实际情况修改返回的数据结构和字段名

	profile := "你叫Vector，是一个桌面机器人，可以与人交互。请用简短的句子回复。\n\n"
	messages := []Message{
		{Role: "user", Content: profile + question},
	}
	domain := "generalv2"
	if vars.APIConfig.Knowledge.RobotName == "api30" {
		domain = "generalv3"
	}
	data := map[string]interface{}{ // 根据实际情况修改返回的数据结构和字段名
		"header": map[string]interface{}{ // 根据实际情况修改返回的数据结构和字段名
			"app_id": appid, // 根据实际情况修改返回的数据结构和字段名
		},
		"parameter": map[string]interface{}{ // 根据实际情况修改返回的数据结构和字段名
			"chat": map[string]interface{}{ // 根据实际情况修改返回的数据结构和字段名
				"domain":      domain,    // 根据实际情况修改返回的数据结构和字段名
				"temperature": float64(0.8), // 根据实际情况修改返回的数据结构和字段名
				"top_k":       int64(6),     // 根据实际情况修改返回的数据结构和字段名
				"max_tokens":  int64(2048),  // 根据实际情况修改返回的数据结构和字段名
				"auditing":    "default",    // 根据实际情况修改返回的数据结构和字段名
			},
		},
		"payload": map[string]interface{}{ // 根据实际情况修改返回的数据结构和字段名
			"message": map[string]interface{}{ // 根据实际情况修改返回的数据结构和字段名
				"text": messages, // 根据实际情况修改返回的数据结构和字段名
			},
		},
	}
	return data // 根据实际情况修改返回的数据结构和字段名
}

// 创建鉴权url  apikey 即 hmac username
func assembleAuthUrl1(hosturl string, apiKey, apiSecret string) string {
	ul, err := url.Parse(hosturl)
	if err != nil {
		fmt.Println(err)
	}
	//签名时间
	date := time.Now().UTC().Format(time.RFC1123)
	//date = "Tue, 28 May 2019 09:10:42 MST"
	//参与签名的字段 host ,date, request-line
	signString := []string{"host: " + ul.Host, "date: " + date, "GET " + ul.Path + " HTTP/1.1"}
	//拼接签名字符串
	sgin := strings.Join(signString, "\n")
	// fmt.Println(sgin)
	//签名结果
	sha := HmacWithShaTobase64("hmac-sha256", sgin, apiSecret)
	// fmt.Println(sha)
	//构建请求参数 此时不需要urlencoding
	authUrl := fmt.Sprintf("hmac username=\"%s\", algorithm=\"%s\", headers=\"%s\", signature=\"%s\"", apiKey,
		"hmac-sha256", "host date request-line", sha)
	//将请求参数使用base64编码
	authorization := base64.StdEncoding.EncodeToString([]byte(authUrl))

	v := url.Values{}
	v.Add("host", ul.Host)
	v.Add("date", date)
	v.Add("authorization", authorization)
	//将编码后的字符串url encode后添加到url后面
	callurl := hosturl + "?" + v.Encode()
	return callurl
}

func HmacWithShaTobase64(algorithm, data, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	encodeData := mac.Sum(nil)
	return base64.StdEncoding.EncodeToString(encodeData)
}

func readResp(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("code=%d,body=%s", resp.StatusCode, string(b))
}


type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}


const (
    STATUS_FIRST_FRAME     = 0
    STATUS_CONTINUE_FRAME  = 1
    STATUS_LAST_FRAME      = 2
)

type WsParam struct {
    APPID      string
    APIKey     string
    APISecret  string
    Text       string
    CommonArgs map[string]string
    BusinessArgs map[string]string
    Data       map[string]interface{}
}

func NewWsParam(appid, apikey, apisecret, text string) *WsParam {
    return &WsParam{
        APPID:      appid,
        APIKey:     apikey,
        APISecret:  apisecret,
        Text:       text,
        CommonArgs: map[string]string{"app_id": appid},
        BusinessArgs: map[string]string{"aue": "raw", "auf": "audio/L16;rate=16000", "vcn": "aisbabyxu", "tte": "utf8"},
        Data:       map[string]interface{}{"status": 2, "text": base64.StdEncoding.EncodeToString([]byte(text))},
    }
}

func (wp *WsParam) createURL() string {
    urlx := "wss://tts-api.xfyun.cn/v2/tts"
    now := time.Now().UTC()
    date := now.Format(http.TimeFormat)

    signatureOrigin := "host: " + "ws-api.xfyun.cn" + "\n"
    signatureOrigin += "date: " + date + "\n"
    signatureOrigin += "GET " + "/v2/tts " + "HTTP/1.1"
    h := hmac.New(sha256.New, []byte(wp.APISecret))
    h.Write([]byte(signatureOrigin))
    signatureSha := base64.StdEncoding.EncodeToString(h.Sum(nil))

    authorizationOrigin := fmt.Sprintf("api_key=\"%s\", algorithm=\"%s\", headers=\"%s\", signature=\"%s\"",
        wp.APIKey, "hmac-sha256", "host date request-line", signatureSha)
    authorization := base64.StdEncoding.EncodeToString([]byte(authorizationOrigin))

	v := url.Values{}
	v.Add("host", "ws-api.xfyun.cn")
	v.Add("date", date)
	v.Add("authorization", authorization)

    return urlx + "?" + v.Encode()
}

func xftts(text string) []byte {
    wsParam := NewWsParam(vars.APIConfig.Knowledge.ID, vars.APIConfig.Knowledge.Key, vars.APIConfig.Knowledge.Model, text)
    wsURL := wsParam.createURL()

    c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
    if err != nil {
        fmt.Println("dial:", err)
        return nil
    }
    defer c.Close()

    d := map[string]interface{}{
        "common":    wsParam.CommonArgs,
        "business":  wsParam.BusinessArgs,
        "data":      wsParam.Data,
    }
    msg, _ := json.Marshal(d)
    err = c.WriteMessage(websocket.TextMessage, msg)
    if err != nil {
        fmt.Println("write:", err)
        return nil
    }

	alldata := make([]byte, 0)

    for {
        _, message, err := c.ReadMessage()
        if err != nil {
            fmt.Println("read:", err)
            return nil
        }

        var result map[string]interface{}
        json.Unmarshal(message, &result)
        code := result["code"].(float64)
        if code != 0 {
            errMsg := result["message"].(string)
            fmt.Printf("call error: %s, code: %f\n", errMsg, code)
        } else {
            audio := result["data"].(map[string]interface{})["audio"].(string)
            audioData, _ := base64.StdEncoding.DecodeString(audio)
			alldata = append(alldata, audioData...)
            status := result["data"].(map[string]interface{})["status"].(float64)
            if status == 2 {
                fmt.Println("ws is closed")
				break
            }
        }
    }

	return alldata
}