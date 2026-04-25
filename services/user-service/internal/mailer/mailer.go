package mailer

import (
	"fmt"
	"net/smtp"
	"os"
)

type Mailer struct {
	host     string
	port     string
	username string
	password string
	from     string
}

func New(host, port, username, password string) *Mailer {
	return &Mailer{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     "drova@example.com",
	}
}

func (m *Mailer) SendActivation(toEmail, token string) error {
	addr := fmt.Sprintf("%s:%s", m.host, m.port)
	auth := smtp.PlainAuth("", m.username, m.password, m.host)

	link := fmt.Sprintf("http://localhost:8082/v1/users/activate/%s", token)
	body := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: Drova – Activate your account\r\n\r\n"+
			"Welcome to Drova!\r\n\r\nActivate your account:\r\n%s\r\n\r\nLink expires in 72 hours.",
		m.from, toEmail, link,
	)

	if err := smtp.SendMail(addr, auth, m.from, []string{toEmail}, []byte(body)); err != nil {
		fmt.Fprintf(os.Stdout, `{"level":"info","msg":"activation_link","email":%q,"link":%q}`+"\n", toEmail, link)
		return err
	}
	return nil
}
