//go:build windows

package inject

import (
	"encoding/binary"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32       = windows.NewLazySystemDLL("user32.dll")
	procSendInput = user32.NewProc("SendInput")
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	mouseEventMove       = 0x0001
	mouseEventLeftDown   = 0x0002
	mouseEventLeftUp     = 0x0004
	mouseEventRightDown  = 0x0008
	mouseEventRightUp    = 0x0010
	mouseEventMiddleDown = 0x0020
	mouseEventMiddleUp   = 0x0040
	mouseEventWheel      = 0x0800
	mouseEventHWheel     = 0x1000
	mouseEventAbsolute   = 0x8000

	keyEventKeyUp = 0x0002
)

// inputSize est la taille de la structure Win32 INPUT sur amd64/arm64
// (union MOUSEINPUT/KEYBDINPUT/HARDWAREINPUT alignée sur 8 octets à cause de
// ULONG_PTR dwExtraInfo) : 4 (type) + 4 (padding) + 32 (union) = 40 octets.
// Voir https://learn.microsoft.com/windows/win32/api/winuser/ns-winuser-input
const inputSize = 40

type WindowsProvider struct{}

func NewProvider() Provider { return WindowsProvider{} }

func sendRawInput(buf [inputSize]byte) error {
	ret, _, err := procSendInput.Call(
		uintptr(1),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(inputSize),
	)
	if ret == 0 {
		return err
	}
	return nil
}

func (WindowsProvider) MoveMouse(x, y int32) error {
	var buf [inputSize]byte
	binary.LittleEndian.PutUint32(buf[0:4], inputMouse)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(x))
	binary.LittleEndian.PutUint32(buf[12:16], uint32(y))
	binary.LittleEndian.PutUint32(buf[20:24], mouseEventMove|mouseEventAbsolute)
	return sendRawInput(buf)
}

func (WindowsProvider) MouseButtonEvent(button MouseButton, down bool) error {
	var flags uint32
	switch button {
	case MouseLeft:
		if down {
			flags = mouseEventLeftDown
		} else {
			flags = mouseEventLeftUp
		}
	case MouseRight:
		if down {
			flags = mouseEventRightDown
		} else {
			flags = mouseEventRightUp
		}
	case MouseMiddle:
		if down {
			flags = mouseEventMiddleDown
		} else {
			flags = mouseEventMiddleUp
		}
	}
	var buf [inputSize]byte
	binary.LittleEndian.PutUint32(buf[0:4], inputMouse)
	binary.LittleEndian.PutUint32(buf[20:24], flags)
	return sendRawInput(buf)
}

func (WindowsProvider) MouseWheel(deltaX, deltaY int32) error {
	if deltaY != 0 {
		var buf [inputSize]byte
		binary.LittleEndian.PutUint32(buf[0:4], inputMouse)
		binary.LittleEndian.PutUint32(buf[16:20], uint32(deltaY))
		binary.LittleEndian.PutUint32(buf[20:24], mouseEventWheel)
		if err := sendRawInput(buf); err != nil {
			return err
		}
	}
	if deltaX != 0 {
		var buf [inputSize]byte
		binary.LittleEndian.PutUint32(buf[0:4], inputMouse)
		binary.LittleEndian.PutUint32(buf[16:20], uint32(deltaX))
		binary.LittleEndian.PutUint32(buf[20:24], mouseEventHWheel)
		return sendRawInput(buf)
	}
	return nil
}

func (WindowsProvider) KeyEvent(virtualKeyCode uint16, down bool) error {
	var buf [inputSize]byte
	binary.LittleEndian.PutUint32(buf[0:4], inputKeyboard)
	binary.LittleEndian.PutUint16(buf[8:10], virtualKeyCode)
	if !down {
		binary.LittleEndian.PutUint32(buf[12:16], keyEventKeyUp)
	}
	return sendRawInput(buf)
}
