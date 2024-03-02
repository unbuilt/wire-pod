package processreqs

import (
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/kercre123/wire-pod/chipper/pkg/vtt"
	sr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/speechrequest"
	ttr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/ttr"

	//pb "github.com/digital-dream-labs/api/go/chipperpb"
	"fmt"
	"strings"
	"github.com/fforchino/vector-go-sdk/pkg/vectorpb"
	"github.com/fforchino/vector-go-sdk/pkg/vector"
	sdkWeb "github.com/kercre123/wire-pod/chipper/pkg/wirepod/sdkapp"
	"context"
	"time"
	"encoding/json"
	"io/ioutil"
)


func getSDKSettings(robot *vector.Vector,ctx context.Context) ([]byte, error) {
	resp, err := robot.Conn.PullJdocs(ctx, &vectorpb.PullJdocsRequest{
		JdocTypes: []vectorpb.JdocType{vectorpb.JdocType_ROBOT_SETTINGS},
	})
	if err != nil {
		return nil, err
	}
	json := resp.NamedJdocs[0].Doc.JsonDoc

	// json内容: {
	// 	"button_wakeword" : 0,
	// 	"clock_24_hour" : true,
	// 	"custom_eye_color" : {
	// 	   "enabled" : false,
	// 	   "hue" : 0,
	// 	   "saturation" : 0
	// 	},
	// 	"default_location" : "San Francisco, California, United States",
	// 	"dist_is_metric" : true,
	// 	"eye_color" : 3,
	// 	"locale" : "en-US",
	// 	"master_volume" : 3,
	// 	"temp_is_fahrenheit" : false,
	// 	"time_zone" : "Asia/Hong_Kong"
	//  }


	return []byte(json), nil
}


func RefreshSDKSettings(robot *vector.Vector,ctx context.Context) map[string]interface{} {

	var settings map[string]interface{}

	settingsJSON, err := getSDKSettings(robot,ctx)
	if err != nil {
		logger.Println("ERROR: Could not load Vector settings from JDOCS")
		return settings
	}

	//println(string(settingsJSON))

	json.Unmarshal([]byte(settingsJSON), &settings)
	return settings
}

func play_sound_data(audioData []byte, botSerial string) string {

	robotObj, _, _ := sdkWeb.GetRobot(botSerial)
	robot := robotObj.Vector
	ctx := robotObj.Ctx

	if robot == nil {
		return "intent_imperative_apologize"
	}

	if ctx == nil {
		return "intent_imperative_apologize"
	}

	settings := RefreshSDKSettings(robot,ctx)
	master_volume := int(settings["master_volume"].(float64))
	println("Current Volume:",master_volume)

	start := make(chan bool)
	stop := make(chan bool)
	go func() {
		err := robot.BehaviorControl(ctx, start, stop)
		if err != nil {
			fmt.Println(err)
		}
	}()
	logger.Println("start playing")
	for {
		select {
		case <-start:
			var audioChunks [][]byte
			for len(audioData) >= 1024 {
				audioChunks = append(audioChunks, audioData[:1024])
				audioData = audioData[1024:]
			}
			var audioClient vectorpb.ExternalInterface_ExternalAudioStreamPlaybackClient
			audioClient, _ = robot.Conn.ExternalAudioStreamPlayback(
				ctx,
			)
			audioClient.SendMsg(&vectorpb.ExternalAudioStreamRequest{
				AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamPrepare{
					AudioStreamPrepare: &vectorpb.ExternalAudioStreamPrepare{
						AudioFrameRate: 16000,
						AudioVolume:   20*uint32(master_volume), //0~5 -> 0~100
					},
				},
			})
			
			for _, chunk := range audioChunks {
				audioClient.SendMsg(&vectorpb.ExternalAudioStreamRequest{
					AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamChunk{
						AudioStreamChunk: &vectorpb.ExternalAudioStreamChunk{
							AudioChunkSizeBytes: 1024,
							AudioChunkSamples:   chunk,
						},
					},
				})
				time.Sleep(time.Millisecond * 30)
			}
			audioClient.SendMsg(&vectorpb.ExternalAudioStreamRequest{
				AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamComplete{
					AudioStreamComplete: &vectorpb.ExternalAudioStreamComplete{},
				},
			})
			logger.Println("Played")

			stop <- true
			return "intent_imperative_praise"
		}
	}
}

// This is here for compatibility with 1.6 and older software
func (s *Server) ProcessIntent(req *vtt.IntentRequest) (*vtt.IntentResponse, error) {
	Interrupt(req.Device)
	var successMatched bool
	speechReq := sr.ReqToSpeechRequest(req)
	var transcribedText string
	if !isSti {
		var err error
		transcribedText, err = sttHandler(speechReq)
		if err != nil {
			ttr.IntentPass(req, "intent_system_noaudio", "voice processing error", map[string]string{"error": err.Error()}, true)
			return nil, nil
		}
		successMatched = ttr.ProcessTextAll(req, transcribedText, vars.MatchListList, vars.IntentsList, speechReq.IsOpus)
	} else {
		intent, slots, err := stiHandler(speechReq)
		if err != nil {
			if err.Error() == "inference not understood" {
				logger.Println("No intent was matched")
				ttr.IntentPass(req, "intent_system_noaudio", "voice processing error", map[string]string{"error": err.Error()}, true)
				return nil, nil
			}
			logger.Println(err)
			ttr.IntentPass(req, "intent_system_noaudio", "voice processing error", map[string]string{"error": err.Error()}, true)
			return nil, nil
		}
		ttr.ParamCheckerSlotsEnUS(req, intent, slots, speechReq.IsOpus, speechReq.Device)
		return nil, nil
	}
	if !successMatched {
		if vars.APIConfig.Knowledge.Provider == "spark" {
			RemoveFromInterrupt(req.Device)
			//resp := openaiRequest(transcribedText)
			//logger.LogUI("OpenAI response for device " + req.Device + ": " + resp)
			//KGSim(req.Device, resp)

			// Check if text is empty
			if transcribedText == "" {
				return nil, nil
			}

			// Get Spark response
			apiResponse := sparkProcess(transcribedText, req.Device)

			if (apiResponse != "") {

				audioData := xftts(apiResponse)
				if audioData == nil {
					logger.Println("xftts error")
					return nil, nil
				}

				logger.Println("playing")
				play_sound_data(audioData, req.Device)
				logger.Println("played")

				ttr.IntentPass(req, "intent_imperative_praise", transcribedText, map[string]string{"": ""}, false)
				return nil, nil
			}
		}

		logger.Println("No intent was matched.")
		ttr.IntentPass(req, "intent_system_noaudio", transcribedText, map[string]string{"": ""}, false)
		return nil, nil
	}
	logger.Println("Bot " + speechReq.Device + " request served.")
	return nil, nil
}


func sparkProcess(transcribedText string, device string) string {
	logger.Println("Sparking...")

	useVision := false
	if (strings.Contains(transcribedText, "你看")) {
		useVision = true
	}

	apiResponse := ""

	if (!useVision) {
		// Get Spark response
		apiResponse = sparkRequest(transcribedText)
		logger.Println("Spark response: " + apiResponse)

	} else {
		robotObj, _, _ := sdkWeb.GetRobot(device)
		robot := robotObj.Vector
		ctx := robotObj.Ctx
	
		if robot == nil {
			return ""
		}
	
		if ctx == nil {
			return ""
		}

		ir, err := robot.Conn.CaptureSingleImage(ctx, &vectorpb.CaptureSingleImageRequest{})
		logger.Println("Captured")
		//logger.Println(ir)
		logger.Println(err)
		// Save image
		image_path := "/tmp/image.jpg"
		ioutil.WriteFile(image_path, ir.GetData(), 0644)
		logger.Println("Image saved to " + image_path)

		apiResponse = imageUnderstand(ir.GetData(), transcribedText)

		// Replace "你看" with "你看看"
		//apiResponse = strings.Replace(apiResponse, "你看", "你看看", -1)
	}

	return apiResponse
}
