sudo usermod -aG dialout $USER or permissions set on /dev/ttyACM0

conni@herbie:~/work/fcd/github/cbuschka/go-spdsxpro$ amidi -p hw:1,0,0 -S F0411000000000161100000000000000047CF7 -r /dev/stdout -t 2 | xxd
00000000: f041 1000 0000 0016 1200 0000 0000 0002  .A..............
00000010: 0579 f7                                  .y.
