package message

import (
	chatmodel "PProject/module/chat/model"

	"go.mongodb.org/mongo-driver/mongo"
)

type Store struct {
	SeqConvColl     *mongo.Collection // seq_conversation
	ConvColl        *mongo.Collection // conversation
	MsgColl         *mongo.Collection // message
	SparseBlockColl *mongo.Collection // read_sparse_block
	MentionColl     *mongo.Collection // mention_index
}

func NewStore(db *mongo.Database) *Store {
	seq := chatmodel.SeqConversation{}
	cov := chatmodel.Conversation{}
	msg := chatmodel.MessageModel{}
	rsb := chatmodel.ReadSparseBlock{}
	met := chatmodel.MentionIndex{}
	return &Store{
		SeqConvColl:     seq.Collection(),
		ConvColl:        cov.Collection(),
		MsgColl:         msg.Collection(),
		SparseBlockColl: rsb.Collection(),
		MentionColl:     met.Collection(),
	}
}
