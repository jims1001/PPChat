package msgflow

import "go.mongodb.org/mongo-driver/mongo/options"

func mongoIndexOptions(unique bool, name string) *options.IndexOptions {
	opts := options.Index()
	opts.SetUnique(unique)
	opts.SetName(name)
	return opts
}
