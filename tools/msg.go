package tools

import (
	pb "PProject/gen/message"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
)

func EncodeFrame(req *pb.MessageFrameData) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("EncodeFrame: req is nil")
	}
	return protojson.Marshal(req)
}

// DecodeFrame 从 []byte 解析回 MessageFrameData
func DecodeFrame(data []byte) (*pb.MessageFrameData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("DecodeFrame: data is empty")
	}
	var frame pb.MessageFrameData
	if err := protojson.Unmarshal(data, &frame); err != nil {
		return nil, err
	}
	return &frame, nil
}
