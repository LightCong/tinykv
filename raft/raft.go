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

//https://www.cnblogs.com/lygin/p/15141330.html
package raft
//todo 把index 整成从1 开始的，注意len（entris） done
//todo append marjority 还得再看下 done
//持久化 hardstate （term，votefor，commit），log 这些
import (
	"errors"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"math/rand"

	pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"
)

// None is a placeholder node ID used when there is no leader.
const None uint64 = 0

// StateType represents the role of a node in a cluster.
type StateType uint64

const (
	StateFollower StateType = iota
	StateCandidate
	StateLeader
)

var stmap = [...]string{
	"StateFollower",
	"StateCandidate",
	"StateLeader",
}

func (st StateType) String() string {
	return stmap[uint64(st)]
}

// ErrProposalDropped is returned when the proposal is ignored by some cases,
// so that the proposer can be notified and fail fast.
var ErrProposalDropped = errors.New("raft proposal dropped")

// Config contains the parameters to start a raft.
type Config struct {
	// ID is the identity of the local raft. ID cannot be 0.
	ID uint64

	// peers contains the IDs of all nodes (including self) in the raft cluster. It
	// should only be set when starting a new raft cluster. Restarting raft from
	// previous configuration will panic if peers is set. peer is private and only
	// used for testing right now.
	peers []uint64

	// ElectionTick is the number of Node.Tick invocations that must pass between
	// elections. That is, if a follower does not receive any message from the
	// leader of current term before ElectionTick has elapsed, it will become
	// candidate and start an election. ElectionTick must be greater than
	// HeartbeatTick. We suggest ElectionTick = 10 * HeartbeatTick to avoid
	// unnecessary leader switching.
	ElectionTick int
	// HeartbeatTick is the number of Node.Tick invocations that must pass between
	// heartbeats. That is, a leader sends heartbeat messages to maintain its
	// leadership every HeartbeatTick ticks.
	HeartbeatTick int

	// Storage is the storage for raft. raft generates entries and states to be
	// stored in storage. raft reads the persisted entries and states out of
	// Storage when it needs. raft reads out the previous state and configuration
	// out of storage when restarting.
	Storage Storage
	// Applied is the last applied index. It should only be set when restarting
	// raft. raft will not return entries to the application smaller or equal to
	// Applied. If Applied is unset when restarting, raft might return previous
	// applied entries. This is a very application dependent configuration.
	Applied uint64
}

func (c *Config) validate() error {
	if c.ID == None {
		return errors.New("cannot use none as id")
	}

	if c.HeartbeatTick <= 0 {
		return errors.New("heartbeat tick must be greater than 0")
	}

	if c.ElectionTick <= c.HeartbeatTick {
		return errors.New("election tick must be greater than heartbeat tick")
	}

	if c.Storage == nil {
		return errors.New("storage cannot be nil")
	}

	return nil
}

// Progress represents a follower’s progress in the view of the leader. Leader maintains
// progresses of all followers, and sends entries to the follower based on its progress.
type Progress struct {
	Match, Next uint64
}

type Raft struct {
	id uint64 //记录自己的标识

	Term uint64 //全局当前在哪个世代上
	Vote uint64 //记录投票给了谁

	// the log
	RaftLog *RaftLog

	// log replication progress of each peers
	Prs map[uint64]*Progress

	// this peer's role
	State StateType

	// votes records
	votes map[uint64]bool //记录别人是否投票给我了

	// msgs need to send
	msgs []pb.Message

	// the leader id
	Lead uint64

	// heartbeat interval, should send
	heartbeatTimeout int
	// baseline of election interval
	electionTimeout     int
	randElectionTimeout int
	// number of ticks since it reached last heartbeatTimeout.
	// only leader keeps heartbeatElapsed.
	heartbeatElapsed int
	// Ticks since it reached last electionTimeout when it is leader or candidate.
	// Number of ticks since it reached last electionTimeout or received a
	// valid message from current leader when it is a follower.
	electionElapsed int

	// leadTransferee is id of the leader transfer target when its value is not zero.
	// Follow the procedure defined in section 3.10 of Raft phd thesis.
	// (https://web.stanford.edu/~ouster/cgi-bin/papers/OngaroPhD.pdf)
	// (Used in 3A leader transfer)
	leadTransferee uint64

	// Only one conf change may be pending (in the log, but not yet
	// applied) at a time. This is enforced via PendingConfIndex, which
	// is set to a value >= the log index of the latest pending
	// configuration change (if any). Config changes are only allowed to
	// be proposed if the leader's applied index is greater than this
	// value.
	// (Used in 3A conf change)
	PendingConfIndex uint64
}

// newRaft return a raft peer with the given config
func newRaft(c *Config) *Raft {
	if err := c.validate(); err != nil {
		panic(err.Error())
	}
	// Your Code Here (2A).
	raftLog := newLog(c.Storage)
	raftLog.applied = c.Applied
	prs := make(map[uint64]*Progress, len(c.peers))
	for _, pid := range c.peers {
		prs[pid] = &Progress{}
	}
	r := Raft{
		id:               c.ID,
		Term:             0,
		Vote:             0,
		RaftLog:          raftLog,
		Prs:              prs,
		State:            StateFollower,
		Lead:             0,
		msgs:             []pb.Message{},
		heartbeatTimeout: c.HeartbeatTick,
		electionTimeout:  c.ElectionTick,
	}
	return &r
}

//在状态变化时，对raft实例状态做清理
func (r *Raft) initRaft() {
	r.Vote = 0
	r.heartbeatElapsed = 0
	r.electionElapsed = 0
	r.randElectionTimeout = r.electionTimeout + rand.Intn(r.electionTimeout)
	r.Lead = 0
	r.votes = make(map[uint64]bool, len(r.Prs))
	for pid := range r.Prs {
		r.votes[pid] = false
	}
}


// tick advances the internal logical clock by a single tick.
func (r *Raft) tick() {
	// Your Code Here (2A).
	switch r.State {
	case StateFollower, StateCandidate:
		r.electionElapsed += 1
		if r.electionElapsed >= r.randElectionTimeout {
			r.electionElapsed = 0
			//campaign 竞选，本质就是给自己发一条msghup
			localMsg := pb.Message{
				MsgType: pb.MessageType_MsgHup,
			}
			r.Step(localMsg)
		}
	case StateLeader:
		r.heartbeatElapsed += 1
		if r.heartbeatElapsed >= r.heartbeatTimeout {
			r.heartbeatElapsed = 0
			//触发发送心跳
			localMsg := pb.Message{
				MsgType: pb.MessageType_MsgBeat,
			}
			r.Step(localMsg)
		}
	}
}

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
	for id,pr:=range r.Prs {
		if id == r.id {
			continue
		}
		pr.Next = r.RaftLog.LastIndex()+1 //todo next 的语义具体是什么？？
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

// Step the entrance of handle message, see `MessageType`
// on `eraftpb.proto` for what msgs should be handled
func (r *Raft) Step(m pb.Message) error {
	// Your Code Here (2A).
	switch r.State {
	case StateFollower:
		switch m.MsgType {
		//触发竞选
		case pb.MessageType_MsgHup:
			r.becomeCandidate()
			if len(r.Prs) == 1 {
				r.becomeLeader()
			} else {
				for peerID := range r.Prs {
					if peerID == r.id {
						continue
					}
					r.sendVote(peerID)
				}
			}
			// follower 收到一个投票请求
		case pb.MessageType_MsgRequestVote:
			if m.Term > r.Term {
				//r.becomeFollower(m.Term,m.From)
				r.becomeFollowerStateWhenReciveVote(m.Term)
			}
			r.handleVote(m)
			//follower 收到一个心跳消息,如果感知到term 更高，则改为follow 这个lead
		case pb.MessageType_MsgHeartbeat:
			if m.Term > r.Term {
				r.becomeFollower(m.Term, m.From)
			}
			//follower 收到一个msg append 请求，如果感知到是更高term 的leader，则改为follow 这个lead
		case pb.MessageType_MsgAppend:
			if m.Term > r.Term {
				r.becomeFollower(m.Term, m.From)
			}
			r.handleAppendEntries(m)
		default:
			err := fmt.Errorf("invalid state and recive msg %v,%v", r.State, m.MsgType)
			fmt.Println(err.Error())
		}
	case StateCandidate:
		switch m.MsgType {
		case pb.MessageType_MsgHup:
			//触发竞选
			r.becomeCandidate()
			if len(r.Prs) == 1 {
				r.becomeLeader()
			} else {
				for peerID := range r.Prs {
					if peerID == r.id {
						continue
					}
					r.sendVote(peerID)
				}
			}
		//竞选者收到投票请求
		case pb.MessageType_MsgRequestVote:
			if m.Term > r.Term {
				//r.becomeFollower(m.Term,m.From)
				r.becomeFollowerStateWhenReciveVote(m.Term)
			}
			r.handleVote(m)

			//竞选者，收到投票结论
		case pb.MessageType_MsgRequestVoteResponse:
			if m.Reject == false {
				r.votes[m.From] = true
			}
			voteNum := 0
			for _, vote := range r.votes {
				if vote == true {
					voteNum += 1
				}
			}
			if voteNum > len(r.votes)/2 {
				r.becomeLeader()
			}
			//竞选者 收到一个心跳 请求，
			//如果感知到是更高term 的leader，则改为follow 这个lead
			//如果收到了同 term 的心跳，则改为follow 这个lead

			//竞选者收到心跳包
		case pb.MessageType_MsgHeartbeat:
			if m.Term >= r.Term {
				r.becomeFollower(m.Term, m.From)
			}
			//竞选者收到一个msg append 请求，如果感知到是更高term 的leader，则改为follow 这个lead
			//如果收到了同 term 的append气你供求，则改为follow 这个lead
		case pb.MessageType_MsgAppend:
			if m.Term >= r.Term {
				r.becomeFollower(m.Term, m.From)
				r.handleAppendEntries(m)
			}
		default:
			err := fmt.Errorf("invalid state and recive msg %v,%v", r.State, m.MsgType)
			fmt.Println(err.Error())
		}
	case StateLeader:
		switch m.MsgType {
		//本地消息，触发广播心跳
		case pb.MessageType_MsgBeat:
			for peerID := range r.Prs {
				if peerID == r.id {
					continue
				}
				r.sendHeartbeat(peerID)
			}
			//收到propose 请求，触发广播msg append
		case pb.MessageType_MsgPropose:
			//the leader first calls the 'appendEntry' method to append entries to its log
			//then calls 'bcastAppend' method to send those entries to peers
			err := r.leaderAppendEntry(m)
			if err != nil {
				log.Error(err.Error())
				return err
			}
			r.bcastappend()
		//leader 收到了别的candidate 的投票请求
		case pb.MessageType_MsgRequestVote:
			if m.Term > r.Term {
				//r.becomeFollower(m.Term,m.From)
				r.becomeFollowerStateWhenReciveVote(m.Term)
			}
			r.handleVote(m)

			//leader 收到了其他leader 的心跳包，如果term 更高，会变成follower
		case pb.MessageType_MsgHeartbeat:
			if m.Term > r.Term {
				r.becomeFollower(m.Term, m.From)
			}
			//leader 收到了其他leader 的msgappend 包，如果term 更高，会变成follower
		case pb.MessageType_MsgAppend:
			if m.Term > r.Term {
				r.becomeFollower(m.Term, m.From)
				r.handleAppendEntries(m)
			}
			//leader 收到了msgappend 回包，开始处理
		case pb.MessageType_MsgAppendResponse:
			if m.Term > r.Term {
				r.becomeFollower(m.Term, m.From)
			} else {
				r.leaderHandleAppendResp(m)
			}
		case pb.MessageType_MsgHeartbeatResponse:
			//todo leader 处理自己发出去的心跳包的响应
		default:
			err := fmt.Errorf("invalid state and recive msg %v,%v", r.State, m.MsgType)
			fmt.Println(err.Error())
			//panic(err.Error())
		}
	}
	return nil
}







// handleHeartbeat handle Heartbeat RPC request
func (r *Raft) handleHeartbeat(m pb.Message) {
	// Your Code Here (2A).
}

// handleSnapshot handle Snapshot RPC request
func (r *Raft) handleSnapshot(m pb.Message) {
	// Your Code Here (2C).
}

// addNode add a new node to raft group
func (r *Raft) addNode(id uint64) {
	// Your Code Here (3A).
}

// removeNode remove a node from raft group
func (r *Raft) removeNode(id uint64) {
	// Your Code Here (3A).
}


func (r * Raft) debug() {
	fmt.Println("---------------------")
	fmt.Printf("raft state:%+v\n",r)
	for id,pr:=range r.Prs {
		if id == r.id {
			continue
		}
		fmt.Printf("peer id %v, peer process %+v\n",id,pr)
	}
	fmt.Printf("raft log state:%v\n",ltoa(r.RaftLog))
	fmt.Println("---------------------")
}