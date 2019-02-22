echo 'select path,md5_sum from files where (mode & 0x80000000)==0;' | sqlite3 files-20190222.db | sort | sed -e "s?^$(pwd)?.?" > md5s1.txt
echo 'select path,sha256_sum from files where (mode & 0x80000000)==0;' | sqlite3 files-20190222.db | sort | sed -e "s?^$(pwd)?.?" > sha256s1.txt
find . -type f | xargs md5sum | awk '{printf "%s|%s\n",$NF,$1}' | sort > md5s2.txt
find . -type f | xargs sha256sum | awk '{printf "%s|%s\n",$NF,$1}' | sort > sha256s2.txt
