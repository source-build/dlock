package dlock

import (
	"errors"
)

type eventType byte

const (
	nilEvent eventType = iota

	typeErrorEvent

	authEvent   //authentication
	LockEvent   //lock
	UnLockEvent //unlock

	decodeFailEvent     //Decoding failed
	authOKEvent         //Authentication succeeded
	authFailEvent       //Authentication failed
	OperateTimeoutEvent //Operation timeout / lock obtained but not unlocked in time
	LockOKEvent         //Locking succeeded
	LockFailEvent       //Lock failed /busy / lock not obtained
	alreadyLockEvent    //Locked

	UnLockOKEvent    //Unlock succeeded
	notFindLockEvent //No lock but attempt to unlock
)

const (
	flagBit   = 0
	lengthBit = flagBit + 1
)

func encode(event eventType, message ...[]byte) ([]byte, error) {
	pack := make([]byte, 2)
	pack[flagBit] = byte(event)
	if event == authEvent || event == LockEvent {
		if len(message) == 0 || len(message[0]) == 0 {
			return nil, errors.New("in the authentication phase, the 'message' cannot be empty")
		}
		if len(message[0]) > 0xFF {
			return nil, errors.New("the message can only hold one byte in length")
		}
		pack[lengthBit] = byte(len(message[0]))
		pack = append(pack, message[0]...)
	}

	return pack, nil
}

func decode(pack []byte) (eventType, []byte, error) {
	event := pack[flagBit]
	if event > 0xFF {
		return 0, nil, errors.New("parsing failed")
	}
	var resultEvent = nilEvent
	switch eventType(event) {
	case nilEvent:
		resultEvent = nilEvent
	case authEvent:
		resultEvent = authEvent
	case LockEvent:
		resultEvent = LockEvent
	case UnLockEvent:
		resultEvent = UnLockEvent
	case LockOKEvent:
		resultEvent = LockOKEvent
	case UnLockOKEvent:
		resultEvent = UnLockOKEvent
	case OperateTimeoutEvent:
		resultEvent = OperateTimeoutEvent
	case LockFailEvent:
		resultEvent = LockFailEvent
	}
	if resultEvent == nilEvent {
		return nilEvent, nil, errors.New("unrecognized packet")
	}

	length := pack[lengthBit]
	if resultEvent == authEvent || resultEvent == LockEvent {
		if length == 0 {
			return nilEvent, nil, errors.New("packet format error")
		}
		if int(length) != len(pack)-lengthBit-1 {
			return nilEvent, nil, errors.New("unrecognized packet")
		}
		body := pack[lengthBit+1 : lengthBit+1+length]
		return resultEvent, body, nil
	}

	return resultEvent, nil, nil
}
