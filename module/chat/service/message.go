package service

import (
	pb "PProject/gen/message"
	msgModel "PProject/module/chat/model"
	util "PProject/tools"
	errors "PProject/tools/errs"
	ids "PProject/tools/ids"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
)

// 入口：把 pb.MessageData 转成可落库的 MessageModel
func BuildMessageModelFromPB(
	tenantID string,
	md *pb.MessageData,
	seq int64, // 会话内序号（外部计算并传入）
	conversationID string, // 会话ID（单聊=u1:u2；群聊=groupID），外部传入更灵活
) (*msgModel.MessageModel, error) {
	if md == nil {
		return nil, errors.New("nil MessageData")
	}
	now := time.Now().UnixMilli()

	m := &msgModel.MessageModel{
		TenantID:         tenantID,
		ClientMsgID:      md.GetClientMsgId(),
		ServerMsgID:      ids.GenerateString(),
		CreateTimeMS:     util.Nz64(md.GetCreateTime(), now),
		SendTimeMS:       util.Nz64(md.GetSendTime(), now),
		SessionType:      msgModel.SessionType(md.GetSessionType()),
		SendID:           md.GetSendId(),
		RecvID:           md.GetRecvId(),
		MsgFrom:          msgModel.MsgFrom(md.GetMsgFrom()),
		ContentType:      msgModel.ContentType(md.GetContentType()),
		SenderPlatformID: msgModel.PlatformID(md.GetSenderPlatformId()),
		SenderNickname:   md.GetSenderNickname(),
		SenderFaceURL:    md.GetSenderFaceUrl(),
		GroupID:          md.GetGroupId(),
		ConversationID:   conversationID,
		Seq:              seq,
		IsRead:           util.Bool2i(md.GetIsRead()),
		Status:           int(md.GetStatus()),

		GuildID:   md.GetGuildId(),
		ChannelID: md.GetChannelId(),
		ThreadID:  md.GetThreadId(),

		AttachedInfo: md.GetAttachedInfo(),
		Ex:           util.TryParseJSONMap(md.GetEx()), // 如果你 Ex 是 JSON 字符串，按需解析；否则保留字符串
		LocalEx:      md.GetLocalEx(),

		IsEdited:     util.Bool2i(md.GetIsEdited()),
		EditedAtMS:   md.GetEditedAt(),
		EditVersion:  md.GetEditVersion(),
		ExpireAtMS:   md.GetExpireAt(),
		AccessLevel:  md.GetAccessLevel(),
		TraceID:      md.GetTraceId(),
		SessionTrace: md.GetSessionId(),
		Tags:         md.GetTags(),
		ReplyTo:      md.GetReplyTo(),
		IsEphemeral:  util.Bool2i(md.GetIsEphemeral()),
		// Rich / Automod 如需强类型映射，按你的定义处理；这里示例略过
	}

	// OfflinePush 映射（可选）
	if md.OfflinePush != nil {
		m.OfflinePush = &msgModel.OfflinePushInfo{
			Title:                     md.OfflinePush.GetTitle(),
			Desc:                      md.OfflinePush.GetDesc(),
			Ex:                        md.OfflinePush.GetEx(),
			IOSBadgeCountPlus1:        md.OfflinePush.GetIosBadgeCountPlus1(),
			IOSCategory:               md.OfflinePush.GetIosCategory(),
			IosSound:                  md.OfflinePush.GetIosSound(),
			AndroidVivoClassification: md.OfflinePush.GetAndroidVivoClassification(),
		}
	}

	// —— 内容 one-of：根据 content_type 只赋一个 elem，并生成 content_text —— //
	switch msgModel.ContentType(md.GetContentType()) {

	case msgModel.TEXT: // 101
		if md.TextElem == nil {
			return nil, errors.New("content_type=TEXT but text_elem is nil")
		}
		m.TextElem = &msgModel.TextElem{Content: md.TextElem.GetContent()}
		m.ContentText = util.TrimPreview(md.TextElem.GetContent())

	case msgModel.ADVANCED_TEXT: // 102
		if md.AdvancedTextElem == nil {
			return nil, errors.New("content_type=ADVANCED_TEXT but advanced_text_elem is nil")
		}
		m.AdvancedTextElem = &msgModel.AdvancedTextElem{
			Text:              md.AdvancedTextElem.GetText(),
			MessageEntityList: copyEntities(md.AdvancedTextElem.GetMessageEntityList()),
		}
		m.ContentText = util.TrimPreview(md.AdvancedTextElem.GetText())

	case msgModel.MARKDOWN: // 103
		if md.MarkdownTextElem == nil {
			return nil, errors.New("content_type=MARKDOWN but markdown_text_elem is nil")
		}
		m.MarkdownTextElem = &msgModel.MarkdownTextElem{Content: md.MarkdownTextElem.GetContent()}
		m.ContentText = util.TrimPreview(md.MarkdownTextElem.GetContent())

	case msgModel.PICTURE: // 201
		if md.PictureElem == nil {
			return nil, errors.New("content_type=PICTURE but picture_elem is nil")
		}
		m.PictureElem = &msgModel.PictureElem{
			SourcePicture:   copyPic(md.PictureElem.SourcePicture),
			BigPicture:      copyPic(md.PictureElem.BigPicture),
			SnapshotPicture: copyPic(md.PictureElem.SnapshotPicture),
		}
		m.ContentText = "[图片]"

	case msgModel.SOUND: // 202
		if md.SoundElem == nil {
			return nil, errors.New("content_type=SOUND but sound_elem is nil")
		}
		m.SoundElem = &msgModel.SoundElem{
			UUID:      md.SoundElem.GetUuid(),
			SoundPath: md.SoundElem.GetSoundPath(),
			SourceURL: md.SoundElem.GetSourceUrl(),
			DataSize:  md.SoundElem.GetDataSize(),
			Duration:  md.SoundElem.GetDuration(),
			SoundType: md.SoundElem.GetSoundType(),
		}
		m.ContentText = "[语音]"

	case msgModel.VIDEO: // 203
		if md.VideoElem == nil {
			return nil, errors.New("content_type=VIDEO but video_elem is nil")
		}
		m.VideoElem = &msgModel.VideoElem{
			VideoPath:      md.VideoElem.GetVideoPath(),
			VideoUUID:      md.VideoElem.GetVideoUuid(),
			VideoURL:       md.VideoElem.GetVideoUrl(),
			VideoType:      md.VideoElem.GetVideoType(),
			VideoSize:      md.VideoElem.GetVideoSize(),
			Duration:       md.VideoElem.GetDuration(),
			SnapshotPath:   md.VideoElem.GetSnapshotPath(),
			SnapshotUUID:   md.VideoElem.GetSnapshotUuid(),
			SnapshotSize:   md.VideoElem.GetSnapshotSize(),
			SnapshotURL:    md.VideoElem.GetSnapshotUrl(),
			SnapshotWidth:  md.VideoElem.GetSnapshotWidth(),
			SnapshotHeight: md.VideoElem.GetSnapshotHeight(),
			SnapshotType:   md.VideoElem.GetSnapshotType(),
		}
		m.ContentText = "[视频]"

	case msgModel.FILE: // 204
		if md.FileElem == nil {
			return nil, errors.New("content_type=FILE but file_elem is nil")
		}
		m.FileElem = &msgModel.FileElem{
			UUID:      md.FileElem.GetUuid(),
			SourceURL: md.FileElem.GetSourceUrl(),
			FileName:  md.FileElem.GetFileName(),
			FileSize:  md.FileElem.GetFileSize(),
			FileType:  md.FileElem.GetFileType(),
		}
		m.ContentText = "[文件] " + md.FileElem.GetFileName()

	case msgModel.LOCATION: // 205
		if md.LocationElem == nil {
			return nil, errors.New("content_type=LOCATION but location_elem is nil")
		}
		m.LocationElem = &msgModel.LocationElem{
			Description: md.LocationElem.GetDescription(),
			Longitude:   md.LocationElem.GetLongitude(),
			Latitude:    md.LocationElem.GetLatitude(),
		}
		m.ContentText = "[位置] " + md.LocationElem.GetDescription()

	case msgModel.CARD: // 301
		if md.CardElem == nil {
			return nil, errors.New("content_type=CARD but card_elem is nil")
		}
		m.CardElem = &msgModel.CardElem{
			UserID:   md.CardElem.GetUserId(),
			Nickname: md.CardElem.GetNickname(),
			FaceURL:  md.CardElem.GetFaceUrl(),
			Ex:       md.CardElem.GetEx(),
		}
		m.ContentText = "[名片] " + md.CardElem.GetNickname()

	case msgModel.AT_TEXT: // 302
		if md.AtTextElem == nil {
			return nil, errors.New("content_type=AT_TEXT but at_text_elem is nil")
		}
		m.AtTextElem = &msgModel.AtTextElem{
			Text:         md.AtTextElem.GetText(),
			AtUserList:   md.AtTextElem.GetAtUserList(),
			IsAtSelf:     md.AtTextElem.GetIsAtSelf(),
			AtUsersInfo:  copyAtInfos(md.AtTextElem.GetAtUsersInfo()),
			QuoteMessage: copyLite(md.AtTextElem.QuoteMessage),
		}
		m.ContentText = util.TrimPreview(md.AtTextElem.GetText())

	case msgModel.FACE: // 303
		if md.FaceElem == nil {
			return nil, errors.New("content_type=FACE but face_elem is nil")
		}

		value, _ := util.StructToJSONString(md.FaceElem.GetData())

		m.FaceElem = &msgModel.FaceElem{
			Index: md.FaceElem.GetIndex(),
			Data:  util.TryParseJSONMap(value), // 如果 pb 是 Struct
		}
		m.ContentText = "[表情]"

	case msgModel.MERGE: // 304
		if md.MergeElem == nil {
			return nil, errors.New("content_type=MERGE but merge_elem is nil")
		}
		m.MergeElem = &msgModel.MergeElem{
			Title:             md.MergeElem.GetTitle(),
			AbstractList:      md.MergeElem.GetAbstractList(),
			MultiMessage:      copyLiteSlice(md.MergeElem.GetMultiMessage()),
			MessageEntityList: copyEntities(md.MergeElem.GetMessageEntityList()),
		}
		m.ContentText = "[合并转发] " + md.MergeElem.GetTitle()

	case msgModel.QUOTE: // 305
		if md.QuoteElem == nil {
			return nil, errors.New("content_type=QUOTE but quote_elem is nil")
		}
		m.QuoteElem = &msgModel.QuoteElem{
			Text:         md.QuoteElem.GetText(),
			QuoteMessage: copyLite(md.QuoteElem.QuoteMessage),
		}
		// 文本预览用当前 text；也可拼接被引摘要
		m.ContentText = util.TrimPreview(md.QuoteElem.GetText())

	case msgModel.TYPING: // 306
		if md.TypingElem == nil {
			return nil, errors.New("content_type=TYPING but typing_elem is nil")
		}

		//m.TypingElem = &msgModel.TypingElem{MsgTips: md.TypingElem.GetMsgTips()}
		m.ContentText = "[正在输入]"

	case msgModel.CUSTOM: // 399
		if md.CustomElem == nil {
			return nil, errors.New("content_type=CUSTOM but custom_elem is nil")
		}
		m.CustomElem = &msgModel.CustomElem{
			Data:        util.AnyToMap(md.CustomElem.GetData()), // pb.Struct -> map
			Description: md.CustomElem.GetDescription(),
			Extension:   md.CustomElem.GetExtension(),
		}
		m.ContentText = "[自定义] " + md.CustomElem.GetDescription()

	case msgModel.MsgNOTIFICATION: // 401
		if md.NotificationElem == nil {
			return nil, errors.New("content_type=MsgNOTIFICATION but notification_elem is nil")
		}
		m.NotificationElem = &msgModel.NotificationElem{Detail: md.NotificationElem.GetDetail()}
		m.ContentText = "[通知]"

	default:
		return nil, errors.New("unsupported content_type")
	}

	return m, nil
}

func copyPic(p *pb.PictureBaseInfo) *msgModel.PictureBaseInfo {
	if p == nil {
		return nil
	}
	return &msgModel.PictureBaseInfo{
		UUID: p.GetUuid(), Type: p.GetType(), Size: p.GetSize(),
		Width: p.GetWidth(), Height: p.GetHeight(), URL: p.GetUrl(),
	}
}

func copyEntities(src []*pb.MessageEntity) []*msgModel.MessageEntity {
	if len(src) == 0 {
		return nil
	}
	out := make([]*msgModel.MessageEntity, 0, len(src))
	for _, e := range src {
		if e == nil {
			continue
		}
		out = append(out, &msgModel.MessageEntity{
			Type: e.GetType(), Offset: e.GetOffset(), Length: e.GetLength(),
			URL: e.GetUrl(), Ex: e.GetEx(),
		})
	}
	return out
}

func copyLite(m *pb.MessageData) *msgModel.MessageLite {
	if m == nil {
		return nil
	}
	ret := &msgModel.MessageLite{
		ServerMsgID: m.GetServerMsgId(),
		ContentType: msgModel.ContentType(m.GetContentType()),
		SendTimeMS:  m.GetSendTime(),
		SendID:      m.GetSendId(),
		RecvID:      m.GetRecvId(),
		SessionType: msgModel.SessionType(m.GetSessionType()),
	}
	if m.TextElem != nil {
		ret.TextElem = &msgModel.TextElem{Content: m.TextElem.GetContent()}
	}
	return ret
}

func copyLiteSlice(list []*pb.MessageData) []*msgModel.MessageLite {
	if len(list) == 0 {
		return nil
	}
	out := make([]*msgModel.MessageLite, 0, len(list))
	for _, x := range list {
		out = append(out, copyLite(x))
	}
	return out
}

func copyAtInfos(src []*pb.AtInfo) []*msgModel.AtInfo {
	if len(src) == 0 {
		return nil
	}
	out := make([]*msgModel.AtInfo, 0, len(src))
	for _, a := range src {
		if a == nil {
			continue
		}
		out = append(out, &msgModel.AtInfo{
			AtUserID:      a.GetAtUserId(),
			GroupNickname: a.GetGroupNickname(),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func BuildSyncFrameConversations(userID string,
	convList []*msgModel.Conversation,
) (*pb.MessageFrameData, error) {
	now := time.Now().UnixMilli()

	// 转换 Conversation -> pb.Conversation
	items := make([]*pb.Conversation, 0, len(convList))
	for _, c := range convList {
		if c == nil {
			continue
		}
		items = append(items, &pb.Conversation{
			TenantId:         c.TenantID,
			OwnerUserId:      c.OwnerUserID,
			ConversationId:   c.ConversationID,
			ConversationType: c.ConversationType,
			UserId:           c.UserID,
			GroupId:          c.GroupID,

			ReadSeq:       c.ReadSeq,
			ReadOutboxSeq: c.ReadOutboxSeq,
			LocalMaxSeq:   c.LocalMaxSeq,
			MinSeq:        c.MinSeq,
			ServerMaxSeq:  c.ServerMaxSeq,

			MentionUnread:    c.MentionUnread,
			MentionReadSeq:   c.MentionReadSeq,
			PerDeviceReadSeq: c.PerDeviceReadSeq,

			UpdatedAt: c.UpdatedAt.UnixMilli(),
		})
	}

	payload := &pb.SyncConversations{Conversations: items}
	anyPayload, err := anypb.New(payload)
	if err != nil {
		return nil, err
	}

	frame := &pb.MessageFrameData{
		Type:      pb.MessageFrameData_SYNC,
		From:      "im_server",
		To:        userID,
		Ts:        now,
		GatewayId: "", // 暂时不需要
		ConnId:    "", //暂时不需要
		SessionId: "", //暂时不需要
		// SYNC 通常需要客户端 CACK
		AckRequired: true,
		Body: &pb.MessageFrameData_AnyPayload{
			AnyPayload: anyPayload,
		},
	}

	return frame, nil
}
