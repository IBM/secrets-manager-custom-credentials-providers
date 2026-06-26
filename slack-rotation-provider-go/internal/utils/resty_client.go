package utils

import "github.com/go-resty/resty/v2"

type RestyClientIntf interface {
	Post(authToken string, body interface{}, url string) (*resty.Response, error)
	PostWithFormData(data map[string]string, response interface{}, url string) (*resty.Response, error)
	Delete(authToken string, url string) (*resty.Response, error)
}

type RestyClientStruct struct {
	Client *resty.Client
}

func (r *RestyClientStruct) Post(authToken string, body interface{}, url string) (*resty.Response, error) {
	if authToken != "" {
		return r.Client.R().
			SetAuthToken(authToken).
			SetBody(body).
			Post(url)
	}

	return r.Client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(url)
}
func (r *RestyClientStruct) PostWithFormData(data map[string]string, response interface{}, url string) (*resty.Response, error) {
	return r.Client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetMultipartFormData(data).
		SetResult(response).
		SetDebug(true).
		Post(url)
}

func (r *RestyClientStruct) Delete(authToken string, url string) (*resty.Response, error) {
	return r.Client.R().
		SetAuthToken(authToken).
		Delete(url)
}
