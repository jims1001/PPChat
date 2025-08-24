package global

type Msg struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func Sucess(data any) *Msg {
	return &Msg{
		Code: 200,
		Msg:  "",
		Data: data,
	}
}
