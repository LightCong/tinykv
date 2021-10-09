package standalone_storage

import (
	"github.com/Connor1996/badger"
	"github.com/pingcap-incubator/tinykv/kv/util/engine_util"
)

type standaloneStorageRreader struct {
	txn *badger.Txn
}

func (ssr *standaloneStorageRreader) GetCF(cf string, key []byte) ([]byte, error) {
	val, err := engine_util.GetCFFromTxn(ssr.txn, cf, key)
	if err != nil && err == badger.ErrKeyNotFound {
		return nil,nil
	}
	return val, err
}

func (ssr *standaloneStorageRreader) IterCF(cf string) engine_util.DBIterator {
	return engine_util.NewCFIterator(cf, ssr.txn)
}

func (ssr *standaloneStorageRreader) Close() {
	ssr.txn.Discard()
}
