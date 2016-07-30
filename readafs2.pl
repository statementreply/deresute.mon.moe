use common::sense;
use Data::Dump::Streamer;

my $filename = "tmp/AwbFile";
my $outdir = "tmp2";

if (@ARGV == 2) {
    $filename = shift;
    $outdir = shift;
} else {
    die "$0 filename outdir";
}

my $filesize = (-s $filename);
open my $fh, "<", $filename;
my $buf;
read $fh, $buf, $filesize;
sub at {
    substr $buf, $_[0];
}


my ($magic, $version, $fileCount, $byteAlign) = unpack "a4NVV", at(0);
print "$magic $version, $fileCount $byteAlign\n";
printf "%x\n", $version;
my $offsetFieldSize = 4;

my $FileOffsetLast = unpack "V", at(16 + $fileCount * 2 + $offsetFieldSize * $fileCount);

printf "last %x\n", $FileOffsetLast;

my $PrevFileOffset = 0;
my $PrevCueID;
for (my $i = 0; $i < $fileCount; $i++) {
    my $CueID = unpack "v", at(16 + 2*$i);
    my $FileOffsetRaw = unpack "V", at(16 + $fileCount * 2 + $offsetFieldSize * $i);
    my $FileOffset = $FileOffsetRaw & 0xffffffff;
    my $FileOffsetRound = $FileOffset;

    if ( $FileOffset % $byteAlign ) {
        $FileOffsetRound += $byteAlign - $FileOffset % $byteAlign;
    }



    print "cueid $CueID\n";
    printf "%x %x %x\n", $FileOffset, $FileOffsetRaw, $FileOffsetRound;
    print "\n";

    if ($PrevFileOffset > 0) {
        print Dump([$PrevCueID, $PrevFileOffset, $FileOffsetRound]);
        dump_file($PrevCueID, $PrevFileOffset, $FileOffsetRound);
    }
    $PrevFileOffset = $FileOffsetRound;
    $PrevCueID = $CueID;
}
print Dump([$PrevCueID, $PrevFileOffset, $FileOffsetLast]);
dump_file($PrevCueID, $PrevFileOffset, $FileOffsetLast);

sub dump_file {
    my $cueid = shift;
    my $start = shift;
    my $end = shift;
    write_file(sprintf("tmp3/%03d.hca", $cueid), substr(at($start), 0, $end - $start));
}



sub write_file {
    my $filename = shift;
    print "write to $filename\n";
    my $buf = shift;
    open my $fh, ">", $filename;
    print $fh $buf;
    close $fh;
}

