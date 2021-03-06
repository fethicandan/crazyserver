# crazyserver

_A cross-platform, install-less, dependency-less server for a fleet of Crazyflies_

The crazyserver is written in Go, enabling it to be cross-platform, install-less and dependency-less. It provides a clean, programming-language-agnostic interface to controlling a fleet of Crazyflies by exposing a REST API for Crazyflie configuration and TCP sockets for real-time control.

## Status

Working:

- Parameters
- Logging
- Setpoints
- Console

In Progress:

- REST / TCP Interface
- Flashing (incl. bulk flashing)

TODO:

- Test!!!
- API for MATLAB, Python, Node.js, C/C++

## Crazyradio driver

On Linux and Mac, no driver is needed.

On Windows, you need to install the **winusb** driver with [zadig](http://zadig.akeo.ie/).
