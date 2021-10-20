package raft

import pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"


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

// handleHeartbeat handle Heartbeat RPC request
func (r *Raft) handleHeartbeat(m pb.Message) {
	// Your Code Here (2A).
	msg := pb.Message{
		MsgType: pb.MessageType_MsgHeartbeatResponse,
		From:    r.id,
		To:      m.From,
		Term:    r.Term,
	}
	if r.State != StateFollower ||  m.Term < r.Term {
		msg.Reject = true
	} else {
		msg.Reject = false
		r.electionElapsed = 0
	}
	r.msgs = append(r.msgs, msg)
	return
}