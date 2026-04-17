package mail

import (
	"fmt"
	"html"
)

func PasswordResetEmail(resetLink string) string {
	safeLink := html.EscapeString(resetLink)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ru">
<head><meta charset="UTF-8"></head>
<body style="font-family:Arial,sans-serif;max-width:600px;margin:0 auto;padding:20px;color:#333">
  <h2 style="color:#2563eb">SmartHeart — Сброс пароля</h2>
  <p>Вы получили это письмо, потому что был запрошен сброс пароля для вашей учётной записи.</p>
  <p>Нажмите на кнопку ниже, чтобы установить новый пароль. Ссылка действительна <strong>15 минут</strong>.</p>
  <p style="text-align:center;margin:30px 0">
    <a href="%s"
       style="background:#2563eb;color:#fff;padding:12px 32px;text-decoration:none;border-radius:6px;font-size:16px;display:inline-block">
       Сбросить пароль
    </a>
  </p>
  <p style="font-size:13px;color:#666">Если вы не запрашивали сброс пароля, просто проигнорируйте это письмо.</p>
  <hr style="border:none;border-top:1px solid #eee;margin:30px 0">
  <p style="font-size:12px;color:#999">SmartHeart — Анализ ЭКГ с помощью ИИ</p>
</body>
</html>`, safeLink)
}
