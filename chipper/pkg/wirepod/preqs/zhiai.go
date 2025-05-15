package processreqs

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
	"net/http"

	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	//"github.com/kercre123/wire-pod/chipper/pkg/vars"
	sr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/speechrequest"
	"github.com/gorilla/websocket"
	"gopkg.in/hraban/opus.v2"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"

	"github.com/fforchino/vector-go-sdk/pkg/vectorpb"
	"github.com/fforchino/vector-go-sdk/pkg/vector"
	sdkWeb "github.com/kercre123/wire-pod/chipper/pkg/wirepod/sdkapp"
	"context"
)



// Need a state machine for Xiaozhi STT
// Define the states
type DeviceState int
const (
	Starting DeviceState = iota
	Connecting
	Idle
	Listening
	Speaking
	Finalizing
)

func XiaozhiSTT(req sr.SpeechRequest) (string, error) {
	// Use WebSocket to send audio data to the server
	// and receive the transcription result

	host := "ws://localhost:8000"
	//host := "wss://api.tenclass.net/xiaozhi/v1/"
	dialer := websocket.Dialer{}
	dialer.HandshakeTimeout = 5 * time.Second

	// Set headers for the WebSocket connection
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Device-ID", "8d:cd:62:27:1d:e4")//req.Device)
	headers.Set("Client-ID", "test-client-id")
	headers.Set("Protocol-Version", "1")
	headers.Set("Authorization", "Bearer " + "test-token")

	// Connect to the WebSocket server
	conn, _, err := dialer.Dial(host, headers)
	if err != nil {
		logger.Println("Dial error:", err)
		return "", err
	}

	defer conn.Close()

	// Send the Hello message to the server
	// {"type":"hello","version":1,"transport":"websocket","audio_params":{"format":"opus","sample_rate":16000,"channels":1,"frame_duration":60}}
	helloMessage := map[string]interface{}{
		"type":           "hello",
		"version":        1,
		"transport":      "websocket",
		"audio_params":   map[string]interface{}{"format": "opus", "sample_rate": 16000, "channels": 1, "frame_duration": 60},
	}
	err = conn.WriteJSON(helloMessage)
	if err != nil {
		logger.Println("Write error:", err)
		return "", err
	}

	// Read the server's response
	_, message, err := conn.ReadMessage()
	if err != nil {
		logger.Println("Read error:", err)
		return "", err
	}
	logger.Println("Server response:", string(message))
	// 

	// {"type": "hello", "version": 1, "transport": "websocket", "audio_params": {"format": "opus", "sample_rate": 16000, "channels": 1, "frame_duration": 60}, "session_id": "44d40a28-445b-44e5-8c87-9c3009c0abc7"}
	// Extract the session ID from the server's response
	var helloResponse map[string]interface{}
	err = json.Unmarshal(message, &helloResponse)
	if err != nil {
		logger.Println("Unmarshal error:", err)
		return "", err
	}
	sessionID, _ := helloResponse["session_id"].(string)


	// Send the listen/detect message
	// {"session_id":"a9b643ab-6107-4854-bfaf-fef5e734c4bd","type":"listen","state":"detect","text":"你好小智"}
	//listenDetectMessage := map[string]interface{}{
	//	"session_id": sessionID,
	//	"type":       "listen",
	//	"state":      "detect",
	//	"text":       "你好小智",
	//}
	//err = conn.WriteJSON(listenDetectMessage)
	//if err != nil {
	//	logger.Println("Write error:", err)
	//	return "", err
	//}


	// Send listen/start message
	// {"session_id":"a9b643ab-6107-4854-bfaf-fef5e734c4bd","type":"listen","state":"start","mode":"auto"}
	listenStartMessage := map[string]interface{}{
		"session_id": sessionID,
		"type":       "listen",
		"state":      "start",
		"mode":       "auto",
	}
	err = conn.WriteJSON(listenStartMessage)
	if err != nil {
		logger.Println("Write error:", err)
		return "", err
	}

	opusEncoder, err := opus.NewEncoder(16000, 1, opus.AppAudio)
	if err != nil {
		logger.Println("Encoder error:", err)
		return "", err
	}

	saveAudio := false
	
	// Send the audio data to the server
	// Keep sending audio data in parallel
	// until the server sends a "final" message
	// In a separate goroutine
	go func() {
		var errcount = 0
		for {
			chunk, err := req.GetNextStreamChunk()
			if err != nil {
				logger.Println("GetNextStreamChunk error:", err)
				errcount++
				if errcount > 3 {
					logger.Println("Too many errors, breaking out of loop")
					break
				} else {
					time.Sleep(30 * time.Millisecond)
					continue
				}
			}

			//logger.Println("Chunk length:", len(chunk))

			// has to be split into 320 []byte chunks for VAD
			speechIsDone, _ := req.DetectEndOfSpeech()
			if speechIsDone {
				break
			}			

			// Convert the audio data to OPUS format
			// chunk is in PCM format in bytes
			int16Data := make([]int16, len(chunk)/2)
			for i := 0; i < len(chunk)/2; i++ {
				int16Data[i] = int16(chunk[i*2]) | int16(chunk[i*2+1])<<8
			}

			//logger.Println("Int16 data length:", len(int16Data))

			// Encoded buffer
			opusData := make([]byte, 4096)

			// Encode the audio data to OPUS format
			n, err := opusEncoder.Encode(int16Data, opusData)
			if err != nil {
				logger.Println("Encode error:", err)
				break
			}
			//logger.Println("Encoded data length:", n)

			finalOpusData := opusData[:n]

			// Send the audio data to the server
			err = conn.WriteMessage(websocket.BinaryMessage, finalOpusData)
			if err != nil {
				logger.Println("Write error:", err)
				break
			}
			//logger.Println("Sent audio data to server")

			// Check if the server has sent a "final" message
			// If so, break the loop
			// You can also check for a "partial" message here
			// and handle it accordingly
			// For now, just sleep for a short duration
			time.Sleep(50 * time.Millisecond)
		}

		// Send listen/stop message
		// {"session_id":"a9b643ab-6107-4854-bfaf-fef5e734c4bd","type":"listen","state":"stop"}
		listenStopMessage := map[string]interface{}{
			"session_id": sessionID,
			"type":       "listen",
			"state":      "stop",
		}
		err = conn.WriteJSON(listenStopMessage)
		if err != nil {
			logger.Println("Write error:", err)
			return
		}
		logger.Println("Sent listen/stop message to server")
	}()

	audioFilename := "audio" + sessionID + ".wav"
	rawAudioFilename := "raw_audio" + sessionID + ".raw"

	opusDecoder, err := opus.NewDecoder(16000, 1)
	if err != nil {
		logger.Println("Decoder error:", err)
		return "", err
	}

	var wavWriter *wav.Encoder 
	wavWriter = nil

	if saveAudio {
		// Create a WAV file writer
		wavFile, err := os.Create(audioFilename)
		if err != nil {
			logger.Println("Create WAV file error:", err)
			return "", err
		}
		defer wavFile.Close()

		wavWriter = wav.NewEncoder(wavFile, 16000, 16, 1, 1)
		if err != nil {
			logger.Println("WAV writer error:", err)
			return "", err
		}
		defer wavWriter.Close()

		rawAudioFilename := "raw_audio" + sessionID + ".raw"
		rawAudioFile, err := os.Create(rawAudioFilename)
		if err != nil {
			logger.Println("Create raw audio file error:", err)
			return "", err
		}
		defer rawAudioFile.Close()
	}

	var audioClient vectorpb.ExternalInterface_ExternalAudioStreamPlaybackClient
	audioClient = nil

	// Use a queue to store audio data, not a channel
	var qq [][]byte
	speaking := false
	should_return := false
	should_exit := true

	// Keep getting results until the server sends a "final" message
	// If the message is text, check if it is "final" or "partial"
	// If it is audio data, save it to a file
	for {
		// Read the server's response
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			logger.Println("Read error:", err)
			return "", err
		}
		//logger.Println("Message type:", messageType)
		//logger.Println("Message:", string(message))
		//logger.Println("Message length:", len(message))
		if messageType == websocket.BinaryMessage {
			// Handle audio data
			//logger.Println("Received audio data")
			
			// Decoding the OPUS audio data to PCM format
			decodedData := make([]int16, 4096)
			n, err := opusDecoder.Decode(message, decodedData)
			//logger.Println("Decoded audio data length:", n)
			if err != nil {
			 	logger.Println("Decode error:", err)
				return "", err
			}
		
			// Convert the decoded data to IntBuffer
			wavData := make([]int, n)
			for i := 0; i < n; i++ {
				wavData[i] = int(decodedData[i]*3)
			}

			// Write the raw audio data to a file
			rawAudioBytes := make([]byte, n*2)
			for i := 0; i < n; i++ {
				rawAudioBytes[i*2] = byte(decodedData[i])
				rawAudioBytes[i*2+1] = byte(decodedData[i] >> 8)
			}

			if speaking {
				// Append the raw audio data to the queue
				qq = append(qq, rawAudioBytes[:960])
				qq = append(qq, rawAudioBytes[960:])
				should_exit = false
			}

			if saveAudio {
				// Append the raw audio data to the file
				rawAudioFile, err := os.OpenFile(rawAudioFilename, os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					logger.Println("Open raw audio file error:", err)
					return "", err
				}
				defer rawAudioFile.Close()

				_, err = rawAudioFile.Write(rawAudioBytes)
				if err != nil {
					logger.Println("Write raw audio file error:", err)
					return "", err
				}

				intBuffer := &audio.IntBuffer{Data: wavData}
				// Write the PCM data to the WAV file
				err = wavWriter.Write(intBuffer)
				if err != nil {
					logger.Println("Write WAV file error:", err)
					return "", err
				}
			}
			
			//logger.Println("Saved audio data to file:", audioFilename)
			continue
		}

		if messageType != websocket.TextMessage {
			logger.Println("Unknown message type:", messageType)
			continue
		}

		// Process the text message
		// {"type": "tts", "state": "sentence_start", "session_id": "88c4161c-6f36-47a1-b312-03bc4fbd26bf", "text": "\u4f60\u662f\u4e0d\u662f\u5728\u5916\u9762\u6709\u522b\u7684AI\u4e86"}

		logger.Println("Server response:", string(message))
		var response map[string]interface{}
		err = json.Unmarshal(message, &response)
		if err != nil {
			logger.Println("Unmarshal error:", err)
			return "", err
		}

		//logger.Println("Response:", response)
		//logger.Println("Response type:", response["type"])
		//logger.Println("Response state:", response["state"])
		//logger.Println("Response session_id:", response["session_id"])
		//logger.Println("Response text:", response["text"])
		
		if response["type"] == "tts" {
			if response["state"] == "sentence_start" {
				logger.Println("Received sentence start")
				audioFilename = response["session_id"].(string) + ".wav"
			} else if response["state"] == "sentence_end" {
				logger.Println("Received sentence end")
			} else if response["state"] == "start" {
				logger.Println("Received start")
				audioClient = start_play(req.Device)
				speaking = true
				if audioClient != nil {
					go func() {
						logger.Println("Starting playback")
						for {
							if len(qq) > 0 {
								//logger.Println("Audio queue length:", len(qq))
								qd := qq[0]
								qq = qq[1:]
								//logger.Println("Playing audio data")
								continue_play(audioClient, qd)
								time.Sleep(30 * time.Millisecond)
							}							
							//select {
							//case audioData := <-audioQueue:
							//	logger.Println("Playing audio data")
							//	continue_play(audioClient, audioData)
							//}

							
							// Check if the audioQueue is empty
							// If it is, stop the playback
							// and break the loop
							//logger.Println("Audio queue length:", len(audioQueue))
							//logger.Println("Audio queue:", audioQueue)

							if len(qq) == 0 {
								//logger.Println("Audio queue is nil or empty")
								//logger.Println("speaking:", speaking)
								if !speaking {
									logger.Println("Stopping playback")
									stop_play(audioClient)
									audioClient = nil
									should_return = true
									robotObj, _, _ := sdkWeb.GetRobot(req.Device)
									robot := robotObj.Vector									
									if !should_exit {
										robot.Conn.AppIntent(context.Background(), &vectorpb.AppIntentRequest{Intent: "knowledge_question"})
										logger.Println("Continuing conversation")
									} else {
										robot.Conn.AppIntent(context.Background(), &vectorpb.AppIntentRequest{Intent: "greeting_goodbye"})
										logger.Println("Exiting conversation")
									}
									break
								} else {
									//logger.Println("Audio queue is empty, but still speaking")
									// continue playing
									time.Sleep(20 * time.Millisecond)
									continue
								}
							}
						}
					}()
				}
			} else if response["state"] == "stop" {
				logger.Println("Received stop")
				speaking = false
				break
			}
		}

		time.Sleep(2 * time.Millisecond)

		if should_return {
			logger.Println("Should return")
			break
		}
	}

	logger.Println("Finalizing...")

	return "我有一个问题", nil
}

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

func start_play(botSerial string) vectorpb.ExternalInterface_ExternalAudioStreamPlaybackClient {
	robotObj, _, _ := sdkWeb.GetRobot(botSerial)
	robot := robotObj.Vector
	ctx := robotObj.Ctx

	if robot == nil {
		return nil
	}

	if ctx == nil {
		return nil
	}

	settings := RefreshSDKSettings(robot,ctx)
	master_volume := int(settings["master_volume"].(float64))
	println("Current Volume:",master_volume)

	//start := make(chan bool)
	//stop := make(chan bool)
	go func() {
		//err := robot.BehaviorControl(ctx, start, stop)
		//if err != nil {
		//	fmt.Println(err)
		//}
	}()

	go func() {
		robot.Conn.PlayAnimation(
			context.Background(),
			&vectorpb.PlayAnimationRequest{
				Animation: &vectorpb.Animation{
					Name: "anim_knowledgegraph_searching_01",
				},
				Loops: 3,
			},
		)
	}()

	logger.Println("start playing")
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

	return audioClient
}

func continue_play(audioClient vectorpb.ExternalInterface_ExternalAudioStreamPlaybackClient, chunk []byte) {
	audioClient.SendMsg(&vectorpb.ExternalAudioStreamRequest{
		AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamChunk{
			AudioStreamChunk: &vectorpb.ExternalAudioStreamChunk{
				AudioChunkSizeBytes: 960,
				AudioChunkSamples:   chunk,
			},
		},
	})
}

func stop_play(audioClient vectorpb.ExternalInterface_ExternalAudioStreamPlaybackClient) {
	audioClient.SendMsg(&vectorpb.ExternalAudioStreamRequest{
		AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamComplete{
			AudioStreamComplete: &vectorpb.ExternalAudioStreamComplete{},
		},
	})
}

func xplay_sound_data(audioData []byte, botSerial string) string {

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

	go func() {
		robot.Conn.PlayAnimation(
			context.Background(),
			&vectorpb.PlayAnimationRequest{
				Animation: &vectorpb.Animation{
					Name: "anim_knowledgegraph_searching_01",
				},
				Loops: 8,
			},
		)
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
