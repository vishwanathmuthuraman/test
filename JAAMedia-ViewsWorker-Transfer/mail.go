package main

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	url2 "net/url"
	"os"
	"strconv"
	"time"
)

func SendEmail(viewCount int, url string, discord *discordgo.Session, discordChannelId string) error {
	from := mail.NewEmail("Video Notifications", "atulya@jaamediamarketing.com")
	subject := "Video reached " + strconv.Itoa(viewCount) + " views"
	to := mail.NewEmail("JAA Media", "data@jaamediamarketing.com")
	htmlContent := fmt.Sprintf(`Video with url <a href="%s">%s</a> has reached %d views. <br/><br/> <a href="%s">View metrics</a> <br/> <br/> Please make sure to check comments if you haven’t. Comment team please comment more so new comments appear.`, url, url, viewCount, "https://jmmadmin.com/grafana/d/sponsor-video-details?var-video_url="+url2.QueryEscape(url))
	message := mail.NewSingleEmail(from, subject, to, htmlContent, htmlContent)
	client := sendgrid.NewSendClient(os.Getenv("EMAIL_PASSWORD"))
	ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
	_, err := client.SendWithContext(ctx, message)
	if err != nil {
		return err
	} else {
		//fmt.Println(response.StatusCode)
		//fmt.Println(response.Body)
		//fmt.Println(response.Headers)
	}

	_, err = discord.ChannelMessageSend(discordChannelId, "Video with url "+url+" has reached "+strconv.Itoa(viewCount)+" views. Please make sure to check comments. Comment team please comment more so new comments appear.", discordgo.WithContext(ctx))
	if err != nil {
		fmt.Println("[Discord Error]", err.Error())
	}

	return nil
}
