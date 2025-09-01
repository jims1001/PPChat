package tools

import (
	pb "PProject/gen/message"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
)

func ParseFrameJSON(raw []byte) (*pb.MessageFrameData, error) {
	frame := &pb.MessageFrameData{}
	um := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := um.Unmarshal(raw, frame); err != nil {
		return nil, fmt.Errorf("unmarshal frame failed: %w", err)
	}
	return frame, nil
}
