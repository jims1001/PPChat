package seq

import "go.mongodb.org/mongo-driver/mongo/options"

func mongoIndexOptions(unique bool, name string) *options.IndexOptions {
	opts := options.Index()
	opts.SetUnique(unique)
	opts.SetName(name)
	return opts
}

func normPair(a, b string) (lo, hi string) {
	if a <= b {
		return a, b
	}
	return b, a
}

// 单聊的统一会话ID：p2p:min_max
func buildP2PConvID(a, b string) string {
	lo, hi := normPair(a, b)
	return "p2p:" + lo + "_" + hi
}
