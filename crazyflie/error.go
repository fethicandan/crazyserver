package crazyflie

import "fmt"

type crazyflieError uint8

func (e crazyflieError) Error() string {
	return fmt.Sprintf("crazyflie: %s", radioErrorString[e])
}

const (
	ErrorNoResponse crazyflieError = iota
)

var radioErrorString = map[crazyflieError]string{
	ErrorNoResponse: "not responding",
}
