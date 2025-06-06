package utils

import "github.com/go-resty/resty/v2"

type RestyClientIntf interface {
	Post(authToken string, body interface{}, url string) (*resty.Response, error)
	Delete(authToken string, url string) (*resty.Response, error)
}

type RestyClientStruct struct {
	Client *resty.Client
}

func (r *RestyClientStruct) Post(authToken string, body interface{}, url string) (*resty.Response, error) {
	return r.Client.R().
		SetAuthToken(authToken).
		SetBody(body).
		Post(url)
}

func (r *RestyClientStruct) Delete(authToken string, url string) (*resty.Response, error) {
	return r.Client.R().
		SetAuthToken(authToken).
		Delete(url)
}
