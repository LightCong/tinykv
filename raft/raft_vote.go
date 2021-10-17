package raft

import (
	pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"
)

// sendHeartbeat sends a heartbeat RPC to the given peer.
func (r *Raft) sendHeartbeat(to uint64) {
	// Your Code Here (2A).
	msg := pb.Message{
		MsgType: pb.MessageType_MsgHeartbeat,
		From:    r.id,
		To:      to,
		Term:    r.Term,
	}
	r.msgs = append(r.msgs, msg)
}

func (r *Raft) sendVote(to uint64) {
	// Your Code Here (2A).
	lastIdx := r.RaftLog.LastIndex()
	logTerm, _ := r.RaftLog.Term(lastIdx)

	msg := pb.Message{
		Term:    r.Term,
		MsgType: pb.MessageType_MsgRequestVote,
		From:    r.id,
		To:      to,
		Index:   lastIdx,
		LogTerm: logTerm,
	}
	r.msgs = append(r.msgs, msg)
}

func (r *Raft) msgUptoDate(m pb.Message) bool {
	localLogTerm,_:= r.RaftLog.Term(r.RaftLog.LastIndex())
	if localLogTerm < m.LogTerm {
		return true
	}
	if localLogTerm == m.LogTerm && r.RaftLog.LastIndex() <= m.Index {
		return true
	}
	return false
}

func (r *Raft) handleVote(m pb.Message) {
	if (r.Vote == 0 || r.Vote == m.From) && r.msgUptoDate(m) {
		r.Vote = m.From //给对方投票
		msg := pb.Message{
			MsgType: pb.MessageType_MsgRequestVoteResponse,
			From:    r.id,
			To:      m.From,
			Reject:  false,
			Term:    r.Term,
		}
		r.msgs = append(r.msgs, msg)
	} else {
		msg := pb.Message{
			MsgType: pb.MessageType_MsgRequestVoteResponse,
			From:    r.id,
			To:      m.From,
			Reject:  true,
			Term:    r.Term,
		}
		r.msgs = append(r.msgs, msg)
	}
}
