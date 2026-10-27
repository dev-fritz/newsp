package services

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/dev-fritz/newsp/api/pkg/mail"
)

// emailCopy is the text of one e-mail in one language.
type emailCopy struct {
	Subject string
	Intro   string
	Action  string // button label
	Ignore  string // closing note for people who did not expect the e-mail
}

// emailLocale holds the strings shared by every e-mail of a language.
type emailLocale struct {
	Hello      string // greeting without a name
	HelloName  string // greeting with the name, a fmt format
	Expiry     string // fmt format with the validity
	LinkHint   string
	Hour       string
	Hours      string // fmt format
	Days       string // fmt format
	Copies     map[EmailKind]emailCopy
	HTMLLocale string
}

// defaultEmailLocale is used when the user's locale has no translation.
const defaultEmailLocale = "en"

var emailLocales = map[string]emailLocale{
	"pt-BR": {
		Hello:      "Olá!",
		HelloName:  "Olá, %s!",
		Expiry:     "O link vale por %s e só pode ser usado uma vez.",
		LinkHint:   "Se o botão não funcionar, copie e cole este endereço no navegador:",
		Hour:       "1 hora",
		Hours:      "%d horas",
		Days:       "%d dias",
		HTMLLocale: "pt-BR",
		Copies: map[EmailKind]emailCopy{
			EmailVerification: {
				Subject: "Confirme seu e-mail no newsp",
				Intro:   "Falta pouco: confirme este endereço para começar a receber seus relatórios e ideias.",
				Action:  "Confirmar e-mail",
				Ignore:  "Se você não criou uma conta no newsp, ignore este e-mail.",
			},
			EmailPasswordReset: {
				Subject: "Redefina sua senha do newsp",
				Intro:   "Recebemos um pedido para redefinir a senha da sua conta.",
				Action:  "Redefinir senha",
				Ignore:  "Se você não pediu isso, ignore este e-mail: sua senha continua a mesma.",
			},
			EmailInvite: {
				Subject: "Você recebeu um convite para o newsp",
				Intro:   "Você foi convidado para criar uma conta no newsp, um radar de mercado com relatório diário e ideias de projeto.",
				Action:  "Aceitar convite",
				Ignore:  "Se você não esperava este convite, ignore este e-mail.",
			},
		},
	},
	"en": {
		Hello:      "Hi,",
		HelloName:  "Hi %s,",
		Expiry:     "The link is valid for %s and works only once.",
		LinkHint:   "If the button does not work, paste this address into your browser:",
		Hour:       "1 hour",
		Hours:      "%d hours",
		Days:       "%d days",
		HTMLLocale: "en",
		Copies: map[EmailKind]emailCopy{
			EmailVerification: {
				Subject: "Confirm your e-mail on newsp",
				Intro:   "Almost there: confirm this address to start receiving your reports and ideas.",
				Action:  "Confirm e-mail",
				Ignore:  "If you did not create a newsp account, ignore this e-mail.",
			},
			EmailPasswordReset: {
				Subject: "Reset your newsp password",
				Intro:   "We received a request to reset the password of your account.",
				Action:  "Reset password",
				Ignore:  "If you did not ask for this, ignore this e-mail: your password stays the same.",
			},
			EmailInvite: {
				Subject: "You are invited to newsp",
				Intro:   "You were invited to create an account on newsp, a market radar with a daily report and project ideas.",
				Action:  "Accept invite",
				Ignore:  "If you did not expect this invite, ignore this e-mail.",
			},
		},
	},
}

// emailView is what the templates see.
type emailView struct {
	Lang     string
	Subject  string
	Greeting string
	Intro    string
	Action   string
	Link     string
	Expiry   string
	LinkHint string
	Ignore   string
}

var emailHTML = template.Must(template.New("email").Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Subject}}</title></head>
<body style="margin:0;padding:24px;background:#f5f5f4;font-family:-apple-system,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#1c1917">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0"><tr><td align="center">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:520px;background:#ffffff;border-radius:8px;padding:32px">
<tr><td>
<p style="margin:0 0 24px;font-size:20px;font-weight:600">newsp</p>
<p style="margin:0 0 16px;font-size:16px;line-height:1.5">{{.Greeting}}</p>
<p style="margin:0 0 24px;font-size:16px;line-height:1.5">{{.Intro}}</p>
<p style="margin:0 0 24px"><a href="{{.Link}}" style="display:inline-block;padding:12px 20px;background:#1c1917;color:#ffffff;border-radius:6px;text-decoration:none;font-weight:600">{{.Action}}</a></p>
<p style="margin:0 0 16px;font-size:14px;line-height:1.5;color:#57534e">{{.Expiry}}</p>
<p style="margin:0 0 8px;font-size:14px;line-height:1.5;color:#57534e">{{.LinkHint}}</p>
<p style="margin:0 0 24px;font-size:13px;line-height:1.5;word-break:break-all"><a href="{{.Link}}" style="color:#57534e">{{.Link}}</a></p>
<p style="margin:0;font-size:13px;line-height:1.5;color:#78716c">{{.Ignore}}</p>
</td></tr>
</table>
</td></tr></table>
</body>
</html>
`))

// renderEmail turns e into a message in the recipient's language.
func renderEmail(e Email) (mail.Message, error) {
	if e.To == "" || e.Link == "" {
		return mail.Message{}, fmt.Errorf("render %s email: recipient and link are required", e.Kind)
	}
	loc, ok := emailLocales[e.Locale]
	if !ok {
		loc = emailLocales[defaultEmailLocale]
	}
	c, ok := loc.Copies[e.Kind]
	if !ok {
		return mail.Message{}, fmt.Errorf("render email: unknown kind %q", e.Kind)
	}

	v := emailView{
		Lang: loc.HTMLLocale, Subject: c.Subject, Greeting: loc.Hello, Intro: c.Intro, Action: c.Action,
		Link: e.Link, LinkHint: loc.LinkHint, Ignore: c.Ignore,
		Expiry: fmt.Sprintf(loc.Expiry, loc.duration(e.ValidFor)),
	}
	if name := strings.TrimSpace(e.Name); name != "" {
		v.Greeting = fmt.Sprintf(loc.HelloName, name)
	}

	var html bytes.Buffer
	if err := emailHTML.Execute(&html, v); err != nil {
		return mail.Message{}, fmt.Errorf("render %s email: %w", e.Kind, err)
	}
	text := strings.Join([]string{v.Greeting, v.Intro, v.Action + ": " + v.Link, v.Expiry, v.Ignore}, "\n\n") + "\n"
	return mail.Message{To: e.To, Subject: c.Subject, Text: text, HTML: html.String()}, nil
}

// duration writes a link validity in whole hours, or days from two days up.
func (l emailLocale) duration(d time.Duration) string {
	hours := int(d.Round(time.Hour) / time.Hour)
	switch {
	case hours <= 1:
		return l.Hour
	case hours < 48:
		return fmt.Sprintf(l.Hours, hours)
	default:
		return fmt.Sprintf(l.Days, hours/24)
	}
}
