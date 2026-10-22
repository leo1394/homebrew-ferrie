package selector

import (
    "bufio"
    "os"
    "time"

    "golang.org/x/sys/windows"
)

func enableSelectorTerminal(in, out *os.File) (func(), error) {
    input, output := windows.Handle(in.Fd()), windows.Handle(out.Fd())
    var inputMode, outputMode uint32
    if err := windows.GetConsoleMode(input, &inputMode); err != nil { return nil, err }
    if err := windows.GetConsoleMode(output, &outputMode); err != nil { return nil, err }
    if err := windows.SetConsoleMode(input, inputMode | windows.ENABLE_VIRTUAL_TERMINAL_INPUT); err != nil { return nil, err }
    if err := windows.SetConsoleMode(output, outputMode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
        windows.SetConsoleMode(input, inputMode)
        return nil, err
    }
    return func() {
        windows.SetConsoleMode(input, inputMode)
        windows.SetConsoleMode(output, outputMode)
    }, nil
}

func waitForInput(reader *bufio.Reader, fd int, timeout time.Duration) (bool, error) {
    if reader.Buffered() > 0 { return true, nil }
    if fd < 0 { return false, nil }
    result, err := windows.WaitForSingleObject(windows.Handle(fd), uint32(timeout/time.Millisecond))
    return result == windows.WAIT_OBJECT_0, err
}
