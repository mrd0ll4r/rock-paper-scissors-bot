package bot

import (
	"bitbucket.org/mrd0ll4r/tbotapi"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

type command string

const (
	cmdStart   = command("start")
	cmdStop    = command("stop")
	cmdNew     = command("new")
	cmdJoin    = command("join")
	cmdAbort   = command("abort")
	cmdUnknown = command("unknown")
)

func parseCommand(text string) command {
	text = strings.ToLower(text)
	if !strings.HasPrefix(text, "/") {
		return cmdUnknown
	}

	text = text[1:]
	switch text {
	case "new" || "n" || "new@"+api.Username:
		return cmdNew
	case "join" || "j" || "join@"+api.Username:
		return cmdJoin
	case "abort" || "a" || "abort@"+api.Username:
		return cmdAbort
	case "start" || "start@"+api.Username:
		return cmdStart
	case "stop" || "stop@"+api.Username:
		return cmdStop
	default:
		return cmdUnknown
	}
}

type choice string

const (
	choiceRock     = choice("rock")
	choicePaper    = choice("paper")
	choiceScissors = choice("scissors")
	choiceUnknown  = choice("unknown")
)

func parseChoice(text string) choice {
	text = strings.ToLower(text)
	switch text {
	case "rock" || "r":
		return choiceRock
	case "paper" || "p":
		return choicePaper
	case "scissors" || "s":
		return choiceScissors
	default:
		return choiceUnknown
	}
}

var api *tbotapi.TelegramBotAPI

// RunBot runs a bot.
// It will block until either something very bad happens or closing is closed.
func RunBot(apiKey string, closing chan struct{}) {
	fmt.Println("Starting...")
	if fileExists("chats.json") {
		fmt.Println("Loading chats...")
		err := loadChatsFromFile("chats.json")
		if err != nil {
			fmt.Println("Could not load:", err.Error())
		}
	}

	fmt.Println("Starting bot...")
	api, err := tbotapi.New(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	// just to show its working
	fmt.Printf("User ID: %d\n", api.ID)
	fmt.Printf("Bot Name: %s\n", api.Name)
	fmt.Printf("Bot Username: %s\n", api.Username)

	closed := make(chan struct{})
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-closed:
				return
			case update := <-api.Updates:
				if update.Error() != nil {
					fmt.Printf("Update error: %s\n", update.Error())
					continue
				}

				upd := update.Update()
				switch upd.Type() {
				case tbotapi.MessageUpdate:
					handleMessage(*update.Update().Message, api)
				case tbotapi.InlineQueryUpdate, tbotapi.ChosenInlineResultUpdate:
				// ignore
				default:
					fmt.Printf("Ignoring unknown Update type.")
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-closed:
				return
			case <-time.After(10 * time.Minute):
				fmt.Println("Saving db...")
				err = dumpChatsToFile("chats.json")
				if err != nil {
					fmt.Println("Could not save:", err.Error())
				}
			}
		}
	}()

	// wait for the signal
	<-closing
	fmt.Println("Closing...")

	fmt.Println("Saving db...")
	err = dumpChatsToFile("chats.json")
	if err != nil {
		fmt.Println("Could not save:", err.Error())
	}

	fmt.Println("Closing bot...")
	// always close the API first, let it clean up the update loop
	api.Close() //this might take a while
	close(closed)
	wg.Wait()
}

func fileExists(filename string) bool {
	fi, err := os.Lstat(filename)
	if fi != nil || (err != nil && !os.IsNotExist(err)) {
		return true
	}
	return false
}

func handleMessage(msg tbotapi.Message, api *tbotapi.TelegramBotAPI) {
	typ := msg.Type()
	if typ != tbotapi.TextMessage {
		//ignore non-text messages for now
		return
	}
	text := *msg.Text
	if msg.Chat.IsPrivateChat() {
		fmt.Printf("<-%d, %s,\t%q\n", msg.ID, msg.Chat, text)
	} else {
		fmt.Printf("<-%d, %s(%s),\t%q\n", msg.ID, msg.Chat, msg.From, text)
	}

	if msg.Chat.IsPrivateChat() {
		//always update the list of private chats
		putChat(msg.From.ID, msg.Chat.ID)
	}

	if strings.HasPrefix(text, "/") {
		//command
		cmd := parseCommand(text)
		if cmd == cmdNew {
			game(msg, api)
			return
		}

		groupsLock.RLock()
		if c, ok := groups[msg.Chat.ID]; ok {
			c <- msg
		}
		groupsLock.RUnlock()
	} else {
		if msg.Chat.IsPrivateChat() {
			uid := msg.From.ID
			expectsLock.Lock()
			if expect, ok := expects[uid]; ok {
				switch parseChoice(text) {
				case choiceRock:
					expect <- "rock"
					delete(expects, uid)
				case choicePaper:
					expect <- "paper"
					delete(expects, uid)
				case choiceScissors:
					expect <- "scissors"
					delete(expects, uid)
				default:
					reply(msg, api, "No understand")
				}
			}
			expectsLock.Unlock()
		}
	}
}

func game(msg tbotapi.Message, api *tbotapi.TelegramBotAPI) {
	if hasExpect(msg.From.ID) {
		reply(msg, api, "You are already in a game right now")
		return
	}

	if msg.Chat.IsPrivateChat() {
		reply(msg, api, "You will play against the bot. Make your choice! (reply with rock, paper or scissors)")

		eChan := make(chan string)
		expects[msg.From.ID] = eChan

		go func(original tbotapi.Message, api *tbotapi.TelegramBotAPI, expected chan string) {
			choice := <-eChan

			botChoice := rand.Float64()

			var resp string
			if botChoice < (float64(1) / float64(3)) {
				resp = formatResponse("the bot", "you", "rock", choice)
			} else if botChoice < (float64(2) / float64(3)) {
				resp = formatResponse("the bot", "you", "paper", choice)
			} else {
				resp = formatResponse("the bot", "you", "scissors", choice)
			}

			reply(original, api, resp)
		}(msg, api, eChan)

	} else {
		//group mode

		if !hasChat(msg.From.ID) {
			reply(msg, api, "I have lost track of our private chat. Please write me personally and try again")
			return
		}

		if hasGroup(msg.Chat.ID) {
			reply(msg, api, "This group already has an open game.")
			return
		}

		messages := make(chan tbotapi.Message)
		groups[msg.Chat.ID] = messages
		reply(msg, api, "Game opened. Join with /join, abort with /abort")

		go func(original tbotapi.Message, api *tbotapi.TelegramBotAPI, messages chan tbotapi.Message) {
			var p1, p2 chan string
			var partner tbotapi.User

		loop:
			for {
				msg := <-messages
				if msg.Type() != tbotapi.TextMessage {
					continue
				}
				text := *msg.Text
				switch parseCommand(text) {
				case cmdJoin:
					if msg.From.ID == original.From.ID {
						reply(original, api, "The creator is already in the game, idiot")
					} else {
						expectsLock.Lock()
						if _, ok := expects[original.From.ID]; ok {
							reply(msg, api, "The creator is already in a game right now, game will remain open...")
							expectsLock.Unlock()
							continue loop
						}

						if _, ok := expects[msg.From.ID]; ok {
							reply(msg, api, "You are already in a game right now, game will remain open...")
							expectsLock.Unlock()
							continue loop
						}

						if !hasChat(msg.From.ID) {
							reply(msg, api, "I have lost track of our private chat. Please write me personally and try again")
							expectsLock.Unlock()
							continue loop
						}

						groupsLock.Lock()
						delete(groups, original.Chat.ID)
						groupsLock.Unlock()
						p1 = make(chan string)
						expects[original.From.ID] = p1
						p2 = make(chan string)
						expects[msg.From.ID] = p2
						expectsLock.Unlock()

						p1Chat := chats[original.From.ID]
						p2Chat := chats[msg.From.ID]

						partner = msg.From

						sendTo(p1Chat, api, "Waiting for your choice. ([r]ock, [p]aper, [s]cissors)")
						sendTo(p2Chat, api, "Waiting for your choice. ([r]ock, [p]aper, [s]cissors)")
						reply(original, api, "Game started, send me your choices in a private chat.")

						break loop
					}
				case cmdAbort:
					if msg.From.ID == original.From.ID {
						groupsLock.Lock()
						delete(groups, original.Chat.ID)
						groupsLock.Unlock()
						reply(original, api, "Game aborted.")
						return
					} else {
						reply(original, api, "Only the creator can abort a game")
					}
				}
			}

			// game running

			var choice1, choice2 string

		nextloop:
			for {
				select {
				case choice1 = <-p1:
					if choice2 != "" {
						break nextloop
					}
				case choice2 = <-p2:
					if choice1 != "" {
						break nextloop
					}
				}
			}

			resp := formatResponse(original.From.String(), partner.String(), choice1, choice2)

			reply(original, api, resp)

		}(msg, api, messages)
	}
}

func formatResponse(part1, part2, choice1, choice2 string) string {
	var res string
	switch choice1 {
	case "rock":
		switch choice2 {
		case "rock":
			res = "tie"
		case "paper":
			res = part2 + " wins"
		case "scissors":
			res = part1 + " wins"
		}
	case "paper":
		switch choice2 {
		case "rock":
			res = part1 + " wins"
		case "paper":
			res = "tie"
		case "scissors":
			res = part2 + " wins"
		}
	case "scissors":
		switch choice2 {
		case "rock":
			res = part2 + " wins"
		case "paper":
			res = part1 + " wins"
		case "scissors":
			res = "tie"
		}
	}

	return fmt.Sprintf("%s chose %s, %s chose %s: %s", part1, choice1, part2, choice2, res)
}

func sendTo(chat int, api *tbotapi.TelegramBotAPI, text string) error {
	outMsg, err := api.NewOutgoingMessage(tbotapi.NewChatRecipient(chat), text).Send()
	if err != nil {
		return err
	}

	fmt.Printf("->%d, %s,\t%q\n", outMsg.Message.ID, outMsg.Message.Chat, *outMsg.Message.Text)
	return nil
}

func reply(msg tbotapi.Message, api *tbotapi.TelegramBotAPI, text string) error {
	return sendTo(msg.Chat.ID, api, text)
}
