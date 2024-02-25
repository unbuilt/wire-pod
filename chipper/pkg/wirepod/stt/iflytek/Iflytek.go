package wirepod_iflytek

import (
    "net/url"
    "fmt"
    "time"
    "strings"
    "encoding/base64"
    "crypto/hmac"
    "crypto/sha256"
    "io/ioutil"
    "github.com/gorilla/websocket"
    "net/http"
    "encoding/json"
    "context"

    "github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	sr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/speechrequest"
)

var Name string = "iflytek"

var (
    hostUrl   = "wss://iat-api.xfyun.cn/v2/iat"
	appid     = ""
	apiSecret = ""
    apiKey    = ""    
)

const (
    STATUS_FIRST_FRAME    = 0
    STATUS_CONTINUE_FRAME = 1
    STATUS_LAST_FRAME     = 2
)

func Init() error {
	return nil
}

func STT(req sr.SpeechRequest) (string, error) {
	logger.Println("(Bot " + req.Device + ", Iflytek) Processing...")

    var st=time.Now()
    d := websocket.Dialer{
        HandshakeTimeout: 5 * time.Second,
    }

    //握手并建立websocket 连接
    conn, resp, err := d.Dial(assembleAuthUrl(hostUrl, vars.APIConfig.Knowledge.Key, vars.APIConfig.Knowledge.Model), nil)
    if err != nil {
        panic(readResp(resp)+err.Error())
        return "", nil
    }else if resp.StatusCode !=101{
        panic(readResp(resp)+err.Error())
    }

    //开启协程，发送数据
    context.WithCancel(context.Background())
    defer conn.Close()

	var status = 0
	speechIsDone := false
	req.DetectEndOfSpeech()
	for {
		chunk, err := req.GetNextStreamChunk()
		if err != nil {
			return "", err
		}

		switch status {
            case STATUS_FIRST_FRAME: //发送第一帧音频，带business 参数
                frameData := map[string]interface{}{
                    "common": map[string]interface{}{
                        "app_id": vars.APIConfig.Knowledge.ID, //appid 必须带上，只需第一帧发送
                    },
                    "business": map[string]interface{}{ //business 参数，只需一帧发送
                        "language":"zh_cn",
                        "domain":"iat",
                        "accent":"mandarin",
                    },
                    "data": map[string]interface{}{
                        "status":    STATUS_FIRST_FRAME,
                        "format":    "audio/L16;rate=16000",
                        "audio":     base64.StdEncoding.EncodeToString(chunk),
                        "encoding":  "raw",
                    },
                }
                fmt.Println("send first")
                conn.WriteJSON(frameData)
                status = STATUS_CONTINUE_FRAME
            case STATUS_CONTINUE_FRAME:
                frameData := map[string]interface{}{
                    "data": map[string]interface{}{
                        "status":    STATUS_CONTINUE_FRAME,
                        "format":    "audio/L16;rate=16000",
                        "audio":     base64.StdEncoding.EncodeToString(chunk),
                        "encoding":  "raw",
                    },
                }
				fmt.Println("send continue")
                conn.WriteJSON(frameData)
            case STATUS_LAST_FRAME:
                frameData := map[string]interface{}{
                    "data": map[string]interface{}{
                        "status":    STATUS_LAST_FRAME,
                        "format":    "audio/L16;rate=16000",
                        "audio":     base64.StdEncoding.EncodeToString(chunk),
                        "encoding":  "raw",
                    },
                }
                conn.WriteJSON(frameData)
                fmt.Println("send last")
                break
                //	goto start
            }
		
		if (speechIsDone) {
			break
		}

		// has to be split into 320 []byte chunks for VAD
		speechIsDone = req.DetectEndOfSpeech()
		if speechIsDone {
			status = STATUS_LAST_FRAME
		}
	}

	//获取返回的数据
    var decoder Decoder
    for {
        var resp = RespData{}
        _,msg,err := conn.ReadMessage()
        if err != nil {
            fmt.Println("read message error:", err)
            break
        }
        json.Unmarshal(msg,&resp)
        fmt.Println(string(msg))
        fmt.Println(resp.Data.Result.String(),resp.Sid)
        if resp.Code!=0{
            fmt.Println(resp.Code,resp.Message,time.Since(st))
            break
        }
        decoder.Decode(&resp.Data.Result)
        if resp.Data.Status == 2{
            //cf()
            fmt.Println("final:",decoder.String())
            fmt.Println(resp.Code,resp.Message,time.Since(st))
            
        	return decoder.String(), nil
        }

    }


	return "", nil
}

type RespData struct {
    Sid 	string `json:"sid"`
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    Data  `json:"data"`
}

type Data struct {
    Result Result `json:"result"`
    Status int         `json:"status"`
}

//创建鉴权url  apikey 即 hmac username
func assembleAuthUrl(hosturl string, apiKey, apiSecret string) string {
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
    fmt.Println(sgin)
    //签名结果
    sha := HmacWithShaTobase64("hmac-sha256", sgin, apiSecret)
    fmt.Println(sha)
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
// 解析返回数据，仅供demo参考，实际场景可能与此不同。
type Decoder struct {
    results []*Result
}

func (d *Decoder)Decode(result *Result)  {
    if len(d.results)<=result.Sn{
        d.results = append(d.results,make([]*Result,result.Sn-len(d.results)+1)...)
    }
    if result.Pgs == "rpl"{
        for i:=result.Rg[0];i<=result.Rg[1];i++{
            d.results[i]=nil
        }
    }
    d.results[result.Sn] = result
}

func (d *Decoder)String()string  {
    var r string
    for _,v:=range d.results{
        if v== nil{
            continue
        }
        r += v.String()
    }
    return r
}

type Result struct {
    Ls bool `json:"ls"`
    Rg []int `json:"rg"`
    Sn int `json:"sn"`
    Pgs string `json:"pgs"`
    Ws []Ws `json:"ws"`
}

func (t *Result)String() string {
    var wss string
    for _,v:=range t.Ws{
        wss+=v.String()
    }
    return wss
}

type Ws struct {
    Bg int `json:"bg"`
    Cw []Cw `json:"cw"`
}

func (w *Ws)String()string  {
    var wss string
    for _,v:=range w.Cw{
        wss+=v.W
    }
    return wss
}
type Cw struct {
    Sc int `json:"sc"`
    W string `json:"w"`
}