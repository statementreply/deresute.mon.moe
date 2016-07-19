#!/usr/bin/perl
use common::sense;

my $cached_status = "";

while (sleep 120) {
    my $new_status = qx(curl https://deresuteborder.mon.moe/twitter);
    if ($new_status =~ /^\s*$/ or $new_status =~ /UPDATING/) {
        next;
    }
    if ($new_status ne $cached_status) {
        system qw(perl twitter.pl), $new_status;
        $cached_status = $new_status;
    }
}
