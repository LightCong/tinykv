package raft

import (
	pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"
)

//leaderAppendEntry 将日志添加到自己的log 里
func (r *Raft) leaderAppendEntry(m pb.Message) error {
	for _, ent := range m.Entries {
		newEnt := pb.Entry{
			EntryType: ent.EntryType,
			Data:      ent.Data,
			Term:      r.Term,
			Index:     r.RaftLog.LastIndex()+1,
		}
		r.RaftLog.entries = append(r.RaftLog.entries, newEnt)
	}
	if len(r.Prs) ==1 {
		//直接提交并返回
		r.RaftLog.committed = r.calcLeaderCommitIdx()
	}
	return nil
}

// sendAppend sends an append RPC with new entries (if any) and the
// current commit index to the given peer. Returns true if a message was sent.
func (r *Raft) sendAppend(to uint64) bool {
	// Your Code Here (2A).
	//If last log index ≥ nextIndex for a follower: send AppendEntries RPC with log entries starting at nextIndex
	followerNextIndex := r.Prs[to].Next
	if r.RaftLog.LastIndex() < followerNextIndex {
		return false
	}

	//lastindex >= follower.nextindex && lastindex !=0
	var preLogIndex uint64 = 0
	var preLogTerm uint64 = 0
	if followerNextIndex > 0 {
		preLogIndex = followerNextIndex - 1
		preLogTerm = r.RaftLog.entries[preLogIndex].Term
	}
	tmpents := r.RaftLog.entries[followerNextIndex:]
	ents := []*pb.Entry{}
	for _, ent := range tmpents {
		tmpent:=ent
		ents = append(ents, &tmpent)
	}


	msg := pb.Message{
		MsgType: pb.MessageType_MsgAppend,
		From:    r.id,
		To:      to,
		Term:    r.Term,
		Commit:  r.RaftLog.committed,
		Entries: ents,
		//previous log term and index
		LogTerm: preLogTerm,
		Index:   preLogIndex,
	}
	r.msgs = append(r.msgs, msg)
	return true
}

func (r * Raft) bcastappend() error {
	for peerID := range r.Prs {
		if peerID == r.id {
			continue
		}
		r.sendAppend(peerID)
	}
	return nil
}

// handleAppendEntries handle AppendEntries RPC request for follower
func (r *Raft) handleAppendEntries(m pb.Message) {
	// Your Code Here (2A).
	msg := pb.Message{
		MsgType: pb.MessageType_MsgAppendResponse,
		From:    r.id,
		To:      m.From,
		Term:    r.Term,
	}

	//todo 不匹配时，返回点什么信息，帮助leader 补齐日志？
	if m.Term < r.Term {
		msg.Reject = true
		r.msgs = append(r.msgs, msg)
		return
	}
	preLogTerm := m.LogTerm
	preLogIndex := m.Index

	if r.logMatch(preLogTerm, preLogIndex) == false {
		msg.Reject = true
		r.msgs = append(r.msgs, msg)
		return
	}
	//截断不合法日志
	r.RaftLog.entries = r.RaftLog.entries[:preLogIndex+1]
	for _, ent := range m.Entries {
		r.RaftLog.entries = append(r.RaftLog.entries, *ent)
	}

	//If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
	if m.Commit > r.RaftLog.committed {
		r.RaftLog.committed = min(m.Commit, r.RaftLog.LastIndex())
	}

	msg.Reject = false
	msg.Index = r.RaftLog.LastIndex()
	msg.Commit = r.RaftLog.committed
	r.msgs = append(r.msgs, msg)
	return
}

func (r *Raft) leaderHandleAppendResp(m pb.Message) {
	//If AppendEntries fails because of log inconsistency: decrement nextIndex and retry (§5.3)
	if m.Reject == true {
		r.Prs[m.From].Next -= 1
		r.sendAppend(m.From)
		return
	}
	//If successful: update nextIndex and matchIndex for follower (§5.3)
	r.Prs[m.From].Next = m.Index+1
	r.Prs[m.From].Match = m.Index
	//set commit idx
	r.RaftLog.committed = r.calcLeaderCommitIdx()
}
func (r *Raft) calcLeaderCommitIdx() uint64 {
	//If there exists an N such that N > commitIndex, a majority  of matchIndex[i] ≥ N, and log[N].term == currentTerm:
	//set commitIndex = N (§5.3, §5.4).
	commitIdx := r.RaftLog.committed
	for n := r.RaftLog.committed + 1; n <= r.RaftLog.LastIndex(); n++ {
		cnt := 1
		for peerID := range r.Prs {
			if peerID == r.id {
				continue
			}
			if r.Prs[peerID].Match >= n {
				cnt += 1
			}
		}
		if cnt > len(r.Prs)/2 {
			commitIdx = n
		}
	}
	return commitIdx
}

func (r *Raft) logMatch(preLogTerm uint64, preLogIndex uint64) bool {
	//需要从头开始copy日志
	if preLogIndex == 0 && preLogTerm == 0 {
		return true
	}
	if r.RaftLog.LastIndex() < preLogIndex {
		return false
	}
	localTerm, _ := r.RaftLog.Term(preLogIndex)
	if localTerm == preLogTerm {
		return true
	}
	return false
}