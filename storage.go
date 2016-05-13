package bot

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"

	"github.com/mrd0ll4r/tbotapi"
)

var expects = make(map[int]chan choice)         // maps user IDs to channels of expected messages
var chats = make(map[int]int)                   // maps user IDs to private chat IDs
var groups = make(map[int]chan tbotapi.Message) // maps group IDs to channels of expected messages

var expectsLock sync.RWMutex
var chatsLock sync.RWMutex
var groupsLock sync.RWMutex

type chatsSerializable struct {
	Chats map[string]int `json:"chats"`
}

func newChatsSerializable(chats map[int]int) *chatsSerializable {
	serChats := make(map[string]int)
	for k, v := range chats {
		serChats[fmt.Sprint(k)] = v
	}
	return &chatsSerializable{
		Chats: serChats,
	}
}

func dumpChatsToFile(f string) error {
	file, err := os.Create(f)
	if err != nil {
		return err
	}
	defer file.Close()

	return dumpChats(file)
}

func dumpChats(to io.Writer) error {
	chatsLock.RLock()
	defer chatsLock.RUnlock()
	return json.NewEncoder(to).Encode(newChatsSerializable(chats))
}

func loadChatsFromFile(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	return loadChats(f)
}

func loadChats(from io.Reader) error {
	chatsLock.Lock()
	defer chatsLock.Unlock()
	var c chatsSerializable
	err := json.NewDecoder(from).Decode(&c)
	if err != nil {
		return err
	}

	for k, v := range c.Chats {
		i, err := strconv.Atoi(k)
		if err != nil {
			return err
		}
		chats[i] = v
	}
	return nil
}

func hasExpect(id int) bool {
	expectsLock.RLock()
	defer expectsLock.RUnlock()
	_, ok := expects[id]
	return ok
}

func hasChat(id int) bool {
	chatsLock.RLock()
	defer chatsLock.RUnlock()

	_, ok := chats[id]
	return ok
}

func putChat(id, chat int) {
	chatsLock.Lock()
	defer chatsLock.Unlock()

	chats[id] = chat
}

func hasGroup(id int) bool {
	groupsLock.RLock()
	defer groupsLock.RUnlock()

	_, ok := groups[id]
	return ok
}
