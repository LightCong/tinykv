package server

import (
	"context"
	"github.com/pingcap-incubator/tinykv/kv/storage"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	"github.com/pingcap/log"
)

// The functions below are Server's Raw API. (implements TinyKvServer).
// Some helper methods can be found in sever.go in the current directory

// RawGet return the corresponding Get response based on RawGetRequest's CF and Key fields
func (server *Server) RawGet(_ context.Context, req *kvrpcpb.RawGetRequest) (*kvrpcpb.RawGetResponse, error) {
	// Your Code Here (1).
	reader, err := server.storage.Reader(nil)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}
	val, err := reader.GetCF(req.GetCf(), req.GetKey())
	resp := &kvrpcpb.RawGetResponse{
	}
	if val == nil {
		resp.NotFound = true
		return resp, nil
	}
	if err != nil {
		resp.Error = err.Error()
		return resp, nil
	}
	resp.Value = val
	return resp, nil
}

// RawPut puts the target data into storage and returns the corresponding response
func (server *Server) RawPut(_ context.Context, req *kvrpcpb.RawPutRequest) (*kvrpcpb.RawPutResponse, error) {
	// Your Code Here (1).
	// Hint: Consider using Storage.Modify to store data to be modifie
	putItem := storage.Put{
		Key:   req.Key,
		Value: req.Value,
		Cf:    req.Cf,
	}
	batch := []storage.Modify{}
	item := storage.Modify{Data: putItem}
	batch = append(batch, item)
	err := server.storage.Write(nil, batch)
	resp := &kvrpcpb.RawPutResponse{}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp, nil
}

// RawDelete delete the target data from storage and returns the corresponding response
func (server *Server) RawDelete(_ context.Context, req *kvrpcpb.RawDeleteRequest) (*kvrpcpb.RawDeleteResponse, error) {
	// Your Code Here (1).
	// Hint: Consider using Storage.Modify to store data to be deleted
	delItem := storage.Delete{
		Key: req.Key,
		Cf:  req.Cf,
	}
	batch := []storage.Modify{}
	item := storage.Modify{Data: delItem}
	batch = append(batch, item)
	err := server.storage.Write(nil, batch)
	resp := &kvrpcpb.RawDeleteResponse{}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp, nil
}

// RawScan scan the data starting from the start key up to limit. and return the corresponding result
func (server *Server) RawScan(_ context.Context, req *kvrpcpb.RawScanRequest) (*kvrpcpb.RawScanResponse, error) {
	// Your Code Here (1).
	// Hint: Consider using reader.IterCF
	reader, err := server.storage.Reader(nil)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}
	iter:=reader.IterCF(req.GetCf())
	num:=uint32(0)

	resp := & kvrpcpb.RawScanResponse{
		Kvs: []*kvrpcpb.KvPair{},
	}

	for iter.Seek(req.GetStartKey()); iter.Valid(); iter.Next() {
		if num >= req.GetLimit() {
			break
		}
		num+=1
		item := iter.Item()
		key := item.Key()
		val ,_:= item.Value()

		kvPair := &kvrpcpb.KvPair{
			Key: key,
			Value: val,
		}
		resp.Kvs = append(resp.Kvs,kvPair)
	}
	return resp, nil
}
