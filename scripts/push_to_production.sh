rsync  -avz -f"- .git/" -f"+ *" -e ssh . whereismyfox@whereismyfox.com:~/
