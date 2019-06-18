package main

import (
	"chatApp/client/login"
	"chatApp/client/proto"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"encoding/hex"
	"log"
	"sync"
	"time"

	tui "github.com/marcusolsson/tui-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var client proto.BroadcastClient
var wait *sync.WaitGroup

func init() {
	wait = &sync.WaitGroup{}
}

func connect(user *proto.User, ui tui.UI, newMessage *tui.Box) error {
	var streamerror error

	stream, err := client.CreateStream(context.Background(), &proto.Connect{
		User:   user,
		Active: true,
	})

	if err != nil {
		return fmt.Errorf("connection failed: %v", err)
	}

	wait.Add(1)
	go func(str proto.Broadcast_CreateStreamClient) {
		defer wait.Done()

		for {
			msg, err := str.Recv()
			if err != nil {
				streamerror = fmt.Errorf("Error reading message: %v", err)
				break
			}

			theme := tui.NewTheme()
			theme.SetStyle("label.usernameTheme", tui.Style{
				Underline: tui.DecorationOn,
				Bold:      tui.DecorationOn})
			ui.SetTheme(theme)
			ui.Update(func() {
				usernameText := tui.NewLabel(msg.User.Name)
				usernameText.SetStyleName("usernameTheme")
				if user.Id == msg.User.Id {
					newMessage.Append(tui.NewHBox(
						tui.NewPadder(1, 0, tui.NewLabel("You")),
						tui.NewLabel(" : "),
						tui.NewLabel(msg.Content),
						tui.NewSpacer(),
					))
				} else {
					newMessage.Append(tui.NewHBox(
						tui.NewPadder(1, 0, usernameText),
						tui.NewLabel(" : "),
						tui.NewLabel(msg.Content),
						tui.NewSpacer(),
					))
				}

			})

		}
	}(stream)

	return streamerror
}

func main() {

	username := strings.Title(login.GetUserName())

	newMessage := tui.NewVBox()

	chatBoxScroll := tui.NewScrollArea(newMessage)
	chatBoxScroll.SetAutoscrollToBottom(true)

	chatbox := tui.NewVBox(chatBoxScroll)
	chatbox.SetTitle(username)
	chatbox.SetBorder(true)

	input := tui.NewEntry()
	input.SetFocused(true)
	input.SetSizePolicy(tui.Expanding, tui.Maximum)

	msgInputBox := tui.NewVBox(input)
	msgInputBox.SetTitle("Enter Message")
	msgInputBox.SetBorder(true)
	msgInputBox.SetSizePolicy(tui.Expanding, tui.Maximum)

	root := tui.NewVBox(chatbox, msgInputBox)
	root.SetSizePolicy(tui.Expanding, tui.Maximum)

	ui, err := tui.New(root)
	if err != nil {
		log.Fatal(err)
	}

	ui.SetKeybinding("Esc", func() {
		ui.Quit()
		os.Exit(1)
	})

	//********************************************************************
	timestamp := time.Now()
	done := make(chan int)

	id := sha256.Sum256([]byte(timestamp.String() + username))

	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Couldnt connect to service: %v", err)
	}

	client = proto.NewBroadcastClient(conn)
	user := &proto.User{
		Id:   hex.EncodeToString(id[:]),
		Name: username,
	}

	connect(user, ui, newMessage)

	wait.Add(1)
	go func() {
		defer wait.Done()

		input.OnSubmit(func(e *tui.Entry) {
			m := e.Text()
			input.SetText("")
			msg := &proto.Message{
				User:      user,
				Content:   m,
				Timestamp: timestamp.String(),
			}
			_, err := client.BroadcastMessage(context.Background(), msg)
			if err != nil {
				fmt.Printf("Error Sending Message: %v", err)
			}
		})

	}()

	go func() {
		wait.Wait()
		close(done)
	}()

	if err := ui.Run(); err != nil {
		log.Fatal(err)
	}

	<-done

}
