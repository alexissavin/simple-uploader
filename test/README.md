docker-compose up -d

curl -v -k -Ffile=@sample.txt 'http://172.20.0.2:8080/upload?token=d0701056-a62b-11ee-8871-479dcca8074d'
