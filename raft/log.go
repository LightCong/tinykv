// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package raft

import (
	pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"
)

// RaftLog manage the log entries, its struct look like:
//
//  snapshot/first.....applied....committed....stabled.....last
//  --------|------------------------------------------------|
//                            log entries
//
// for simplify the RaftLog implement should manage all log entries
// that not truncated
type RaftLog struct {
	// storage contains all stable entries since the last snapshot.
	storage Storage

	// committed is the highest log position that is known to be in
	// stable storage on a quorum of nodes.
	committed uint64

	// applied is the highest log position that the application has
	// been instructed to apply to its state machine.
	// Invariant: applied <= committed
	applied uint64

	// log entries with index <= stabled are persisted to storage.
	// It is used to record the logs that are not persisted by storage yet.
	// Everytime handling `Ready`, the unstabled logs will be included.
	stabled uint64

	// all entries that have not yet compact.
	entries []pb.Entry

	// the incoming unstable snapshot, if any.
	// (Used in 2C)
	pendingSnapshot *pb.Snapshot

	// Your Data Here (2A).
	PreIndex uint64
}

// newLog returns log using the given storage. It recovers the log
// to the state that it just commits and applies the latest snapshot.
func newLog(storage Storage) *RaftLog {
	// Your Code Here (2A).
	raftLog := RaftLog{
		storage: storage,
		entries: make([]pb.Entry, 1), //todo index 从1 开始
	}
	if storage == nil {
		return &raftLog
	}
	lo, _ := storage.FirstIndex()
	hi, _ := storage.LastIndex()
	entries, err := storage.Entries(lo, hi+1)
	if err != nil {
		panic(err)
	}
	if lo == 0 {
		raftLog.entries[0].Index = 0
	} else {
		raftLog.entries[0].Index = lo - 1
	}
	raftLog.PreIndex = raftLog.entries[0].Index
	raftLog.stabled = hi
	//raftLog.applied = raftLog.entries[0].Index

	for _, ent := range entries {
		raftLog.entries = append(raftLog.entries, ent)
	}

	hardState, _, nerr := storage.InitialState()
	if nerr != nil {
		panic("invalid newRaft state")
	}
	raftLog.committed = hardState.Commit

	return &raftLog
}

// We need to compact the log entries in some point of time like
// storage compact stabled log entries prevent the log entries
// grow unlimitedly in memory
func (l *RaftLog) maybeCompact() {
	// Your Code Here (2C).
}

// unstableEntries return all the unstable entries
func (l *RaftLog) unstableEntries() []pb.Entry {
	// Your Code Here (2A).
	if l.stabled == l.LastIndex() {
		return []pb.Entry{}
	}

	if l.stabled < l.entries[0].Index {
		panic("invalid stable index")
	}
	return l.entries[l.stabled-l.PreIndex+1:]
}

// nextEnts returns all the committed but not applied entries
func (l *RaftLog) nextEnts() (ents []pb.Entry) {
	// Your Code Here (2A).
	if l.applied == l.committed {
		return []pb.Entry{}
	}
	return l.entries[l.applied-l.PreIndex+1 : l.committed-l.PreIndex+1]
}


// LastIndex return the last index of the log entries
func (l *RaftLog) LastIndex() uint64 {
	// Your Code Here (2A).
	return l.PreIndex + uint64(len(l.entries)) - 1

}

// Term return the term of the entry in the given index
func (l *RaftLog) Term(i uint64) (uint64, error) {
	if i < l.PreIndex {
		panic("unsupport right now")
	}
	offset := i - l.PreIndex
	return l.entries[offset].Term, nil
}

func (l *RaftLog) Append(ent pb.Entry) {
	ent.Index = l.LastIndex() + 1
	l.entries = append(l.entries, ent)
}

func (l *RaftLog) Entris(lo uint64, hi uint64) ([]pb.Entry, error) {
	if lo >= l.PreIndex && hi >= lo && hi <= l.LastIndex() {
		ents := l.entries[lo-l.PreIndex : hi-l.PreIndex+1]
		return ents, nil
	}
	panic("unsupport right")
	return nil, nil
}

func (l *RaftLog) Truncate(preLogIndex uint64) {
	//todo 截断不合法日志
	if preLogIndex >= l.PreIndex {
		l.entries = l.entries[:preLogIndex-l.PreIndex+1]
		if l.stabled > preLogIndex {
			l.stabled = preLogIndex
		}
		return
	}
	panic("unsupport right now")
}

func (l *RaftLog) LogMatch(preLogTerm uint64, preLogIndex uint64) bool {
	//需要从头开始copy日志
	if preLogIndex == 0 && preLogTerm == 0 {
		return true
	}
	if preLogIndex < l.PreIndex {
		panic("unsupport right now")
	}

	if l.LastIndex() < preLogIndex {
		return false
	}
	localTerm, _ := l.Term(preLogIndex)
	if localTerm == preLogTerm {
		return true
	}
	return false
}
