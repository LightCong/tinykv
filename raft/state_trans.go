package raft

import pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"

// becomeFollower transform this peer's state to Follower
func (r *Raft) becomeFollower(term uint64, lead uint64) {
	// Your Code Here (2A).
	r.initRaft()
	r.State = StateFollower
	r.Term = term
	r.Lead = lead
}

func (r *Raft) becomeFollowerStateWhenReciveVote(term uint64) {
	// Your Code Here (2A).
	r.initRaft()
	r.State = StateFollower
	r.Term = term
}

func (r *Raft) becomeFollowerWithoutTerm() {
	// Your Code Here (2A).
	r.initRaft()
	r.State = StateFollower
}

// becomeCandidate transform this peer's state to candidate
func (r *Raft) becomeCandidate() {
	r.initRaft()
	r.Term += 1
	//follower将自己的状态变为candidate。
	r.State = StateCandidate
	//投票给自己。
	r.Vote = r.id
	r.votes[r.id] = true
}

// becomeLeader transform this peer's state to leader
func (r *Raft) becomeLeader() {
	r.initRaft()
	r.Lead = r.id
	r.State = StateLeader
	//todo 更新process
	for id, pr := range r.Prs {
		if id == r.id {
			continue
		}
		pr.Next = r.RaftLog.LastIndex() + 1 //todo next 的语义具体是什么？？
		pr.Match = 0
	}

	// Your Code Here (2A).
	// todo NOTE: Leader should propose a noop entry on its term
	m := pb.Message{
		MsgType: pb.MessageType_MsgPropose,
		Term:    r.Term,
		Entries: []*pb.Entry{{EntryType: pb.EntryType_EntryNormal}},
	}
	r.leaderAppendEntry(m)
	r.bcastappend()
}
