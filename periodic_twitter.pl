#!/usr/bin/perl
use common::sense;

if (@ARGV != 3) {
    die "need 3 parameters: cache_filename, url, interval/sec";
}
my $cache_filename = "cached_status";
$cache_filename = @ARGV[0];
my $url = $ARGV[1];
my $interval = $ARGV[2];
my $cached_status = read_file($cache_filename);

while (1) {
    # -s: silent
    my $new_status = qx(curl -s $url);
    if ($new_status =~ /^\s*$/ or $new_status =~ /UPDATING/) {
        # should sleep
        next;
    }
    if ($new_status ne $cached_status) {
        print scalar(localtime), "\n";
        print "update status: $new_status";
        system qw(perl twitter.pl), $new_status;
        $cached_status = $new_status;
        write_file($cache_filename, $cached_status);
    }
} continue {
    sleep $interval;
}

sub read_file {
    my $filename = shift;
    if (! -e $filename) {
        return "";
    }
    open my $fh, "<", $filename;
    my $n = read $fh, my $buf, 500;
    if ($n == 500) {
        print "full\n";
    }
    return $buf;
}

sub write_file {
    my $filename = shift;
    my $buf = shift;
    open my $fh, ">", $filename;
    print $fh $buf;
    close $fh;
}
