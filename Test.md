```go
func RunCommand(name string, arg ...string) (result []string, err error) {

	cmd := exec.Command(name, arg...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	defer out.Close()
	// 命令的错误输出和标准输出都连接到同一个管道
	cmd.Stderr = cmd.Stdout

	if err = cmd.Start(); err != nil {
		return
	}
	buff := make([]byte, 8)

	for {
		len, err := out.Read(buff)
		if err == io.EOF {
			break
		}
		//result = append(result, string(buff[:len]))
		fmt.Print(string(buff[:len]))
	}
	cmd.Wait()
	return
}

func TestStart() {
	strings := []string{"A", "B", "C", "D"}
	for _, e := range strings {
		for i := 0; i < 10; i++ {
			id := fmt.Sprintf("-i %s%d", e, i)

			go func() {
				result, err := RunCommand("sssa.exe", id, "-a 123456", "-sws://127.0.0.1:8900/ws-report")
				if err != nil {
					fmt.Printf("error -> %s\n", err.Error())
				}

				for _, str := range result {
					fmt.Print(str)
				}
				fmt.Println()
			}()
		}
	}
	select {}
}
//生成
	strings := []string{"A", "B", "C", "D"}
	for _, e := range strings {
		for i := 0; i < 10; i++ {
			serverConfig := config2.ServerConfig{Id: fmt.Sprintf("%s%d", e, i), Name: fmt.Sprintf("%s%d", e, i), Group: e, Secret: "123456"}
			CONFIG.Servers = append(CONFIG.Servers, &serverConfig)
		}
	}
	v.WriteConfigAs("test.yaml")
```