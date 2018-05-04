package tcpmvc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
)

const (
	N_TAG int    = 4      //标识长度
	TAG   string = "mvc|" //标识字符串，用来判断是否是支持本方法读取解析
)

const (
	StatusTCPLose          string = "TPC连接丢失;"
	StatusReadError        string = "读取数据失败;"
	StatusReadOver         string = "无更多数据可获取;"
	StatusWriteFail        string = "数据写入失败"
	StatusWriteLengthError string = "数据写入长度错误"
	StatusUnkonwTag        string = "无法解析此数据1;"
	StatusDataLengthLost   string = "无法解析数据大小;"
	StatusDataLengthZero   string = "数据长度为0;"
	StatusDataLengthError  string = "数据读取不完整;"
	StatusUnkonwModel      string = "未知Model"
	StatusUnkonwMethod     string = "未知Method"
)

//一次Tcp数据
type Data struct {
	Model  string
	Method string
	Args   map[string][]byte
}

func NewData() *Data {
	data := &Data{}
	data.Args = make(map[string][]byte)
	return data
}

type Mvc struct {
	conn   *net.TCPConn
	Models map[string]map[string]reflect.Value
	//LoseLink   chan int //断开了连接
	Disconnect func()
}

func New(c *net.TCPConn) *Mvc {
	models := make(map[string]map[string]reflect.Value)
	m := new(Mvc)
	m.conn = c
	m.Models = models
	return m
}

//包含调用的struct的引用
func (m *Mvc) Include(controller interface{}) {
	fv := reflect.ValueOf(controller)
	ct := reflect.Indirect(fv).Type()
	modelName := ct.Name()
	_, ok := m.Models[modelName]
	if ok {
		return
	}
	rt := fv.Type()
	model := make(map[string]reflect.Value, rt.NumMethod())
	for i := 0; i < rt.NumMethod(); i++ {
		methodName := rt.Method(i).Name
		model[methodName] = fv.MethodByName(methodName)
	}
	m.Models[modelName] = model
}

func (m *Mvc) StartHandle() error {
	var outErr error
	for {
		data, err := m.read()
		if err != nil {
			if err.Error() == StatusReadOver {
				continue
			} else {
				outErr = err
				break
			}
		}
		model, ok := m.Models[data.Model]
		if !ok {
			fmt.Printf("tcpmvc:%s:%s\n", StatusUnkonwModel, data.Model)
			continue
		}
		method, ok := model[data.Method]
		if !ok {
			fmt.Printf("tcpmvc:%s:%s\n", StatusUnkonwMethod, data.Method)
			continue
		}
		args := reflect.ValueOf(data.Args)
		go method.Call([]reflect.Value{args})
	}
	return outErr
}

func (m *Mvc) read() (*Data, error) {
	bytes, err := m.readTcpTag()
	if err != nil {
		return nil, err
	}
	var d Data
	err = json.Unmarshal(bytes, &d)
	if err != nil {
		return nil, errors.New("JSON:" + StatusUnkonwTag + err.Error())
	}
	return &d, nil
}

//从TCP中读取本次数据
func (m *Mvc) readTcpTag() ([]byte, error) {
	if m.conn == nil {
		return nil, errors.New(StatusTCPLose)
	}
	tag := make([]byte, N_TAG)
	l, err := m.conn.Read(tag)
	if err != nil {
		if err == io.EOF {
			return nil, errors.New(StatusReadOver)
		}
		return nil, errors.New(StatusReadError + err.Error())
	}
	if l != N_TAG {
		return nil, errors.New("TAG:" + StatusUnkonwTag)
	}
	stag := string(tag)
	switch stag {
	case TAG:
		_, err := m.conn.Read(tag)
		if err != nil {
			return nil, errors.New(StatusDataLengthLost)
		}
		lbody := binary.LittleEndian.Uint32(tag)
		if lbody == 0 {
			return nil, errors.New(StatusDataLengthZero)
		}
		raw := make([]byte, lbody)
		lraw, err := m.conn.Read(raw)
		if err != nil {
			return nil, errors.New(StatusReadError + err.Error())
		}
		if uint32(lraw) != lbody {
			return nil, errors.New(StatusDataLengthError)
		}
		return raw, nil
	default:
		return nil, errors.New("TAG:" + StatusUnkonwTag + stag)
	}
	return nil, errors.New("TAG2:" + StatusUnkonwTag + stag)
}

func (m *Mvc) Write(data *Data) error {
	json, err := json.Marshal(data)
	if err != nil {
		return errors.New("josn fail:" + err.Error())
	}
	buff := bytes.NewBuffer([]byte{})
	//头标签
	binary.Write(buff, binary.LittleEndian, []byte(TAG))
	err = binary.Write(buff, binary.LittleEndian, int32(len(json)))
	if err != nil {
		return errors.New("binary.Write:" + err.Error())
	}
	binary.Write(buff, binary.LittleEndian, json)
	buffBytes := buff.Bytes()
	l, err := m.conn.Write(buffBytes)
	if err != nil {
		return errors.New(StatusWriteFail + err.Error())
	}
	if l != len(buffBytes) {
		return errors.New(StatusWriteLengthError)
	}
	fmt.Printf("成功写入：%s-%s\n", data.Model, data.Method)
	return nil
}
