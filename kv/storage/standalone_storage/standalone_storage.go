package standalone_storage

import (
	"github.com/Connor1996/badger"
	"github.com/pingcap-incubator/tinykv/kv/config"
	"github.com/pingcap-incubator/tinykv/kv/storage"
	"github.com/pingcap-incubator/tinykv/kv/util/engine_util"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	"os"
	"path/filepath"
)

// StandAloneStorage is an implementation of `Storage` for a single-node TinyKV instance. It does not
// communicate with other nodes and all data is stored locally.
type StandAloneStorage struct {
	// Your Data Here (1)
	kvDB   *badger.DB
	config *config.Config
}

func NewStandAloneStorage(conf *config.Config) *StandAloneStorage {
	// Your Code Here (1).
	dbPath := conf.DBPath
	kvPath := filepath.Join(dbPath, "kv")
	os.MkdirAll(kvPath, os.ModePerm)
	kvDB := engine_util.CreateDB(kvPath, false)
	return &StandAloneStorage{kvDB: kvDB, config: conf}
}

func (s *StandAloneStorage) Start() error {
	// Your Code Here (1).
	return nil
}

func (s *StandAloneStorage) Stop() error {
	// Your Code Here (1).
	if err := s.kvDB.Close(); err != nil {
		return err
	}
	return nil
}

func (s *StandAloneStorage) Reader(ctx *kvrpcpb.Context) (storage.StorageReader, error) {
	// Your Code Here (1).
	return &standaloneStorageRreader{s.kvDB.NewTransaction(false)}, nil
}

func (s *StandAloneStorage) Write(ctx *kvrpcpb.Context, batch []storage.Modify) error {
	// Your Code Here (1).
	if len(batch) == 0 {
		return nil
	}
	kvWB := new(engine_util.WriteBatch)
	for _, item := range batch {
		switch item.Data.(type) {
		case storage.Put:
			putItem := item.Data.(storage.Put)
			kvWB.SetCF(putItem.Cf, putItem.Key, putItem.Value)
		case storage.Delete:
			delItem := item.Data.(storage.Delete)
			kvWB.DeleteCF(delItem.Cf, delItem.Key)
		}
	}
	return kvWB.WriteToDB(s.kvDB)
}
