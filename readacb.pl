#!/usr/bin/perl
use common::sense;
use Data::Dump::Streamer;

my $filename = "data/resourcesbeta/storage/dl/resources/High/Sound/Common/v/3428b3a012082796aeb14d8a0412e602";
my $outdir = "tmp";

if (@ARGV == 2) {
    $filename = shift @ARGV;
    $outdir = shift @ARGV;
} else {
    die "$0 filename outdir";
}

my $filesize = (-s $filename);
print "filesize: $filesize\n";

open my $fh, "<", $filename;
my $buf;
read $fh, $buf, $filesize;
sub at {
    substr $buf, $_[0];
}

print "HEADER\n";

my ($magic, $table_size, $unknown_1,
$row_offset, $string_table_offset, $data_offset, $tablename_offset, $n_field, $row_size, $n_row) = unpack "a4NnnNNNnnN", at(0);

$row_offset += 8;
$string_table_offset += 8;
$data_offset += 8;

print $magic, "\n";
print "$table_size\n";
print "$unknown_1\n";
print "row_offset $row_offset, st_offset $string_table_offset, d_off $data_offset, tn_offset $tablename_offset, n_f $n_field, r_s $row_size, n_r $n_row\n";



my $str = unpack "Z*", at($string_table_offset + $tablename_offset);
print "tablename: <$str>\n";

my $cur = 32;

print "ROW\n";
my %map_unpack = (
    0xb => "N", # data
    0xa => "N", # string
    8 => "f",
    6 => "Q",
    5 => "N",
    4 => "N",
    3 => "n",
    2 => "n",
    1 => "C",
    0 => "C",
);
my %size_unpack = (
    "N" => 4,
    "f" => 4,
    "Q" => 8,
    "n" => 2,
    "C" => 1,
);
if ($table_size > 0) {
    for (my $i = 0; $i < $n_row; $i++) {
        my $currentRowBase = $row_size * $i + $row_offset;
        my $currentOffset = $cur;
        my $currentRowOffset = 0;

        for (my $j = 0; $j < $n_field; $j++) {
            my ($typ, $offset) = unpack "CN", at($currentOffset);
            $currentOffset += 5;
            my $name = unpack "Z*", at($string_table_offset + $offset);

            print "type $typ, offset $offset\n";
            print "t2: ", $typ & 0xf0, "\n";
            print "t1: ", $typ & 0x0f, "\n";
            print "rowname: $name\n";

            if (!exists $map_unpack{$typ & 0x0f}) {
                print "NE\n";
            }
            my $type1 = $typ & 0x0f;
            my $type_unpack = $map_unpack{$typ & 0x0f};

            my $c = unpack $type_unpack, at($currentRowBase+$currentRowOffset);
            if ($type1 == 10) {
                my $fv = unpack "Z*", at($string_table_offset + $c);
                print "c: <$fv>\n";
                $currentRowOffset += 4;
            } elsif ($type1 == 11) {
                my $dataSize = unpack "N", at($currentRowBase+$currentRowOffset+4);
                my $dataOffset = $data_offset + $c;
                print "offset $dataOffset, size $dataSize\n";
                my $data = substr(at($dataOffset), 0, $dataSize);
                print "c: ", Dump(substr($data,0, 100)), "\n";
                $currentRowOffset += 8;
                write_file("$outdir/$name", $data);
            } else {
                print "c: <$c>\n";
                $currentRowOffset += $size_unpack{$type_unpack};
            }
            print "\n";
        }
    }
}

close $fh;

sub write_file {
    my $filename = shift;
    print "write to $filename\n";
    my $buf = shift;
    open my $fh, ">", $filename;
    print $fh $buf;
    close $fh;
}


__END__
public const byte COLUMN_TYPE_STRING = 0x0A;
        // 0x09 double?
        public const byte COLUMN_TYPE_FLOAT = 0x08;
        // 0x07 signed 8byte?
        public const byte COLUMN_TYPE_8BYTE = 0x06;
        public const byte COLUMN_TYPE_4BYTE2 = 0x05;
        public const byte COLUMN_TYPE_4BYTE = 0x04;
        public const byte COLUMN_TYPE_2BYTE2 = 0x03;
        public const byte COLUMN_TYPE_2BYTE = 0x02;
        public const byte COLUMN_TYPE_1BYTE2 = 0x01;
        public const byte COLUMN_TYPE_1BYTE = 0x00;




