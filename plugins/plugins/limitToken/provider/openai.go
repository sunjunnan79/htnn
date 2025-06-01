package provider

import (
    "encoding/json"
    "github.com/pkoukk/tiktoken-go"
    "log"
    "strings"
)

type contextKey string // 定义一个私有类型

const OPENAI_PROMPT contextKey = "OPENAI_PROMPT"

type OpenAIAdapter struct{}

type OpenAIChatRequest struct {
    Model            string             `json:"model"`
    Messages         []OpenAIMessage    `json:"messages"`
    Temperature      float64            `json:"temperature,omitempty"`
    TopP             float64            `json:"top_p,omitempty"`
    N                int                `json:"n,omitempty"`
    Stream           bool               `json:"stream,omitempty"`
    Stop             []string           `json:"stop,omitempty"`
    MaxTokens        int                `json:"max_tokens,omitempty"`
    PresencePenalty  float64            `json:"presence_penalty,omitempty"`
    FrequencyPenalty float64            `json:"frequency_penalty,omitempty"`
    LogitBias        map[string]float64 `json:"logit_bias,omitempty"`
    User             string             `json:"user,omitempty"`
}

type OpenAIMessage struct {
    Role    string `json:"role"`    // "system" | "user" | "assistant" | "function"
    Content string `json:"content"` // 消息内容
}

type OpenAIChatResponse struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    Created int64  `json:"created"`
    Model   string `json:"model"`
    Usage   struct {
        PromptTokens     int `json:"prompt_tokens"`
        CompletionTokens int `json:"completion_tokens"`
        TotalTokens      int `json:"total_tokens"`
    } `json:"usage"`
}

type OpenAIStreamResponse struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    Created int64  `json:"created"`
    Model   string `json:"model"`
    Choices []struct {
        Delta struct {
            Content string `json:"content,omitempty"`
            Role    string `json:"role,omitempty"`
        } `json:"delta"`
        Index        int     `json:"index"`
        FinishReason *string `json:"finish_reason"` // 结束时可能是 "stop" 或其他，流中间为 nil
    } `json:"choices"`
}

func (o *OpenAIAdapter) ConvertRequestToHTNN(reqBody []byte) (*HTNNRequest, error) {
    var req OpenAIChatRequest

    if err := json.Unmarshal(reqBody, &req); err != nil {
        return nil, err
    }

    promptToken, err := o.getToken(req.Messages, req.Model)
    if err != nil {
        return nil, err
    }

    return &HTNNRequest{
        Model:       req.Model,
        PromptToken: promptToken,
        Stream:      req.Stream,
        MaxTokens:   req.MaxTokens,
    }, nil
}

func (o *OpenAIAdapter) ConvertStreamChunkFromHTNN(delta *HTNNStreamDelta, chunk []byte) (*HTNNStreamDelta, error) {
    var streamResp OpenAIStreamResponse
    if err := json.Unmarshal(chunk, &streamResp); err != nil {
        return nil, err
    }

    delta.Finish = false

    for _, choice := range streamResp.Choices {
        delta.Deltas = append(delta.Deltas, &Delta{
            Role:    choice.Delta.Role,
            Content: choice.Delta.Content,
        })

        if choice.FinishReason != nil {
            delta.Finish = true
            //获取OPENAI的Prompt数量
            prompts := make([]OpenAIMessage, len(delta.Deltas))
            for i, v := range delta.Deltas {
                prompts[i] = OpenAIMessage{
                    Role:    v.Role,
                    Content: v.Content,
                }
            }

            completionTokens, err := o.getToken(prompts, streamResp.Model)
            if err != nil {
                return nil, err
            }
            delta.CompletionTokens = completionTokens

            return delta, nil
        }
    }

    return delta, nil
}

func (o *OpenAIAdapter) ConvertResponseFromHTNN(respBody []byte) (*HTNNResponse, error) {
    var normalResp OpenAIChatResponse
    if err := json.Unmarshal(respBody, &normalResp); err != nil {
        return nil, err
    }

    return &HTNNResponse{
        Model:            normalResp.Model,
        PromptTokens:     normalResp.Usage.PromptTokens,
        CompletionTokens: normalResp.Usage.CompletionTokens,
        TotalTokens:      normalResp.Usage.TotalTokens,
    }, nil
}

func (o *OpenAIAdapter) getToken(messages []OpenAIMessage, model string) (int, error) {
    tkm, err := tiktoken.EncodingForModel(model)
    if err != nil {
        log.Printf("encoding for model %s: %v", model, err)
        return 0, err
    }

    var tokensPerMessage int
    switch model {
    case "gpt-3.5-turbo-0613",
        "gpt-3.5-turbo-16k-0613",
        "gpt-4-0314",
        "gpt-4-32k-0314",
        "gpt-4-0613",
        "gpt-4-32k-0613":
        tokensPerMessage = 3
    case "gpt-3.5-turbo-0301":
        tokensPerMessage = 4 // every message follows <|start|>{role/name}\n{content}<|end|>\n
    default:
        if strings.Contains(model, "gpt-3.5-turbo") {
            log.Println("warning: gpt-3.5-turbo may update over time. Returning num tokens assuming gpt-3.5-turbo-0613.")
            return o.getToken(messages, "gpt-3.5-turbo-0613")
        } else if strings.Contains(model, "gpt-4") {
            log.Println("warning: gpt-4 may update over time. Returning num tokens assuming gpt-4-0613.")
            return o.getToken(messages, "gpt-4-0613")
        } else {
            err := log.Output(2, "num_tokens_from_messages() is not implemented for model "+model+". See https://github.com/openai/openai-python/blob/main/chatml.md for information.")
            return 0, err
        }
    }

    numTokens := 0
    for _, message := range messages {
        numTokens += tokensPerMessage
        numTokens += len(tkm.Encode(message.Content, nil, nil))
        numTokens += len(tkm.Encode(message.Role, nil, nil))
    }
    numTokens += 3 // every reply is primed with <|start|>assistant<|message|>
    return numTokens, nil
}
