curl -X 'POST' \
'http://localhost:8080/api/v1/auth/login' \
-H 'accept: application/json' \
-H 'Content-Type: application/json' \
-d '{
"password": "your-password-here",
"username": "admin@example.com"
}'