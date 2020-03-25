package zookeeper

import (
	"fmt"
	"testing"
)

func TestState(t *testing.T) {

	if stateNames[StateUnknown] != "StateUnknown" {
		t.Errorf("state %v should be %v", StateUnknown, "StateUnknown")
	}
	if stateNames[StateDisconnected] != "StateDisconnected" {
		t.Errorf("state %v should be %v", StateDisconnected, "StateDisconnected")
	}
	if stateNames[StateConnecting] != "StateConnecting" {
		t.Errorf("state %v should be %v", StateConnecting, "StateConnecting")
	}
	if stateNames[StateAuthFailed] != "StateAuthFailed" {
		t.Errorf("state %v should be %v", StateAuthFailed, "StateAuthFailed")
	}
	if stateNames[StateConnectedReadOnly] != "StateConnectedReadOnly" {
		t.Errorf("state %v should be %v", StateConnectedReadOnly, "StateConnectedReadOnly")
	}
	if stateNames[StateSaslAuthenticated] != "StateSaslAuthenticated" {
		t.Errorf("state %v should be %v", StateSaslAuthenticated, "StateSaslAuthenticated")
	}
	if stateNames[StateConnected] != "StateConnected" {
		t.Errorf("state %v should be %v", StateConnected, "StateConnected")
	}
	if stateNames[StateHasSession] != "StateHasSession" {
		t.Errorf("state %v should be %v", StateHasSession, "StateHasSession")
	}
}

func ExamplePermission() {
	fmt.Println(PermissionRead)
	fmt.Println(PermissionWrite)
	fmt.Println(PermissionCreate)
	fmt.Println(PermissionDelete)
	fmt.Println(PermissionAdmin)
	fmt.Println(PermissionAll)
	//Output:
	//1
	//2
	//4
	//8
	//16
	//31

}

func ExampleMode() {

	fmt.Println(modeNames[ModeLeader])
	fmt.Println(modeNames[ModeFollower])
	fmt.Println(modeNames[ModeStandalone])
	//Output:
	//leader
	//follower
	//standalone

}
