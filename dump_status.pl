#!/usr/bin/perl
# usage: perl twitter.pl "status"  # utf8 encoded
use common::sense;
use warnings;
use strict;
use Net::Twitter;
use YAML qw(LoadFile Dump);
use Encode qw(decode encode);
use utf8;
use File::Basename;
use URI;
use LWP::Simple;
use open qw(:encoding(UTF-8) :std);
use DateTime::Format::Strptime;
use DateTime;

my $config = LoadFile("secret.yaml");
my $nt = Net::Twitter->new(
    "ssl"      => 1,
    "traits"   => ["API::RESTv1_1",],
    "consumer_key"        => $$config{"twitter_consumer_key"},
    "consumer_secret"     => $$config{"twitter_consumer_secret"},
    "access_token"        => $$config{"twitter_access_token"},
    "access_token_secret" => $$config{"twitter_access_token_secret"},
);

my $result;
eval {
    # user_id => "3697513573"
    $result = $nt->user_timeline({
        screen_name => "deresute_border",
        count => 20,
    });
};

my $download_image = 0;

if ($@) {
    print "err: $@\n";
} else {
    print "no err: $result\n";
    print "n_result: ", scalar @$result, "\n";
    for my $status (@$result) {
        my $timestr = $$status{created_at};
        my $id = $$status{id};
        if ($id =~ m{^\d+$}) {
            # ok
        } else {
            die "bad id format $id";
        }
        my $text = $$status{text};
        print "$id:\n";
        parse_status($text, $timestr);
            
        if ($download_image) {
            my $pic = $$status{entities}{media}[0];
            #print Dump($pic);
            #print "\n";
            my $url = URI->new($$pic{media_url_https});

            my $file = "data/twitter/$id.jpg";
            (undef, undef, my $suf) = fileparse($url, qw(.jpg .gif .png));
            print "$url suf $suf\n";
            print "$url $file\n";
            if ( (defined $url) && ($url ne "") && (not (-e $file)) ) {
                getstore($url, $file);
            }
        }   
    }
}


sub parse_status {
    my $strp = DateTime::Format::Strptime->new(
        pattern => '%a %b %d %T %z %Y',
        on_error => 'croak',
    );
    my $text = shift;
    my $timestr = shift;
    print "timestr: $timestr\n";
    my $create_time = $strp->parse_datetime($timestr);
    #print Dump($time);
    print "epoch(): ", $create_time->epoch(), "\n";

    $text =~ s{#デレステ}{}g;
    $text =~ s{https://t\.co/\w+}{}g;

    my @line = split "\n", $text;
    my $timestamp = 0;
    my @border;
    my %info;
    $info{create_time} = $timestr;
    for my $line (@line) {
        if ($line =~ m{^(\w+)\s*[：:]\s*   (\d+)   [(（][+\d]*[)）]$}x) {
            my ($rank, $score) = ($1, $2);
            print "rank $rank score $score <$line>\n";
            $rank =~ s{千位}{001};
            $rank =~ s{万位}{0001};
            push @border, [$rank, $score];
        } elsif ($line =~ m{^\s*$} ) {
            print "ignore <$line>\n";
        } elsif ($line =~ m{^(\d{2})/(\d{2})\s+(\d{2}):(\d{2})$}x) {
            my ($mon, $day, $hr, $min) = ($1, $2, $3, $4);
            my $year = $create_time->year();
            print "m $1 d $2 h $3 m $4 <$line>\n";

            if ($create_time->month() == $mon) {
                $year = $create_time->year();
            } else {
                if ($mon == 12 and $create_time->month() == 1) {
                    $year = $create_time->year()-1;
                } else {
                    $year = $create_time->year();
                }
            }
            if ($min%15 == 0) {
                $min += 2;
            } else {
                die "bad $min";
            }
            my $status_time = DateTime->new(
                year => $year,
                month => $mon,
                day => $day,
                hour => $hr,
                minute => $min,
                time_zone => 'Asia/Tokyo',
            );
            my $timestamp = $status_time->epoch;
            #print "timestamp: $timestamp\n";
            $info{timestamp} = $timestamp;
        } elsif ($line =~ s{\s*【最終結果】}{}) {
            print "title-final <$line>\n";
            $info{is_final} = 1;
            $info{title} = $line;
        } else {
            print "title <$line>\n";
            $info{title} = $line;
        }
    }
    $info{border} = \@border;
    print Dump(\%info);
}
