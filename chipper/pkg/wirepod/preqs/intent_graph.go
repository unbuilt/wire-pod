package processreqs

import (
	pb "github.com/digital-dream-labs/api/go/chipperpb"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/kercre123/wire-pod/chipper/pkg/vtt"
	sr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/speechrequest"
	ttr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/ttr"
)

func (s *Server) ProcessIntentGraph(req *vtt.IntentGraphRequest) (*vtt.IntentGraphResponse, error) {
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
				logger.Println("Bot " + speechReq.Device + " No intent was matched")
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
		logger.Println("No intent was matched.")
		if vars.APIConfig.Knowledge.Enable && len([]rune(transcribedText)) >= 8 {
			if vars.APIConfig.Knowledge.Provider == "openai" {
				apiResponse := openaiRequest(transcribedText)
				response := &pb.IntentGraphResponse{
					Session:      req.Session,
					DeviceId:     req.Device,
					ResponseType: pb.IntentGraphMode_KNOWLEDGE_GRAPH,
					SpokenText:   apiResponse,
					QueryText:    transcribedText,
					IsFinal:      true,
				}
				req.Stream.Send(response)
				return nil, nil
			} if vars.APIConfig.Knowledge.Provider == "spark" {
				logger.Println("Sparking...")

				// Get Spark response
				apiResponse := sparkRequest(transcribedText)
				logger.Println("Spark response: " + apiResponse)

				audioData := xftts(apiResponse)
				if audioData == nil {
					logger.Println("xftts error")
					return nil, nil
				}

				logger.Println("playing")
				play_sound_data(audioData, req.Device)
				logger.Println("plaed")
				return nil, nil
			}
		}
		ttr.IntentPass(req, "intent_system_noaudio", transcribedText, map[string]string{"": ""}, false)
		return nil, nil
	}
	logger.Println("Bot " + speechReq.Device + " request served.")
	return nil, nil
}
