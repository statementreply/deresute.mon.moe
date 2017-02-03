#!/usr/bin/perl
# import data from @deresute_border to data/twitter.db
# this script was not used to import to data/rank.db
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
use DBI;

sub main {
    my $config = LoadFile("secret.yaml");
    my $download_image = 0;
    my $nt = Net::Twitter->new(
        "ssl"      => 1,
        "traits"   => ["API::RESTv1_1",],
        "consumer_key"        => $$config{"twitter_consumer_key"},
        "consumer_secret"     => $$config{"twitter_consumer_secret"},
        "access_token"        => $$config{"twitter_access_token"},
        "access_token_secret" => $$config{"twitter_access_token_secret"},
    );

    my $dbh = DBI->connect("dbi:SQLite:uri=file:data/twitter.db?mode=rwc");
    my $rv;
    $rv = $dbh->do("CREATE TABLE IF NOT EXISTS rank (timestamp TEXT, type INTEGER, rank INTEGER, score INTEGER, viewer_id INTEGER, PRIMARY KEY(timestamp, type, rank));") or warn $dbh->errstr;
    #print "rv $rv\n";
    $rv = $dbh->do("CREATE TABLE IF NOT EXISTS timestamp (timestamp TEXT, PRIMARY KEY('timestamp'));") or warn $dbh->errstr;
    #print "rv $rv\n";

    my $rc;
    $rc  = $dbh->begin_work   or die $dbh->errstr;

    my $max_id;
    $max_id = "738619236688396288"; # FIXME
    my $n_status = 12000;
    my $i_status = 0;
    my $n_batch = 200;

    while ($i_status < $n_status) {
        my $result;
        eval {
            # user_id => "3697513573"
            my $arg = {
                screen_name => "deresute_border",
                count => $n_batch,
            };
            if (defined $max_id) {
                $$arg{max_id} = $max_id;
            }
            $result = $nt->user_timeline($arg);
        };


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
                if (!defined($max_id)) {
                    $max_id = $id;
                } elsif ($id le $max_id) {
                    $max_id = $id;
                }
                my $text = $$status{text};
                print "$id: len:", length($text), "\n";
                my $info = parse_status($text, $timestr);
                update_db($dbh, $info);
                $i_status++;

                if ($download_image) {
                    my $pic = $$status{entities}{media}[0];
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
            if (@$result != $n_batch) {
                last;
            }
        }
    }
    print "total $i_status\n";
    $rc  = $dbh->commit   or die $dbh->errstr;
    $dbh->disconnect;
}

sub parse_status {
    # to return
    my %info;
    # param
    my $text = shift;
    my $timestr = shift;
    $info{create_time} = $timestr;

    my $strp = DateTime::Format::Strptime->new(
        pattern => '%a %b %d %T %z %Y',
        on_error => 'croak',
    );
    #print "timestr: $timestr\n";
    my $create_time = $strp->parse_datetime($timestr);
    #print Dump($time);
    #print "epoch(): ", $create_time->epoch(), "\n";
    $info{create_time_unix} = $create_time->epoch();

    $text =~ s{#デレステ}{}g;
    $text =~ s{https://t\.co/\w+}{}g;

    my @line = split "\n", $text;
    my $timestamp = 0;
    my @border;
    for my $line (@line) {
        if ($line =~ m{^(\w+)\s*[：:]\s*   (\d+)   [(（][+\d]*[)）]$}x) {
            my ($rank, $score) = ($1, $2);
            #print "rank $rank score $score <$line>\n";
            $rank =~ s{千位}{001};
            $rank =~ s{万位}{0001};
            push @border, [$rank, $score];
        } elsif ($line =~ m{^\s*$} ) {
            #print "ignore <$line>\n";
        } elsif ($line =~ m{^(\d{2})/(\d{2})\s+(\d{2}):(\d{2})$}x) {
            my ($mon, $day, $hr, $min) = ($1, $2, $3, $4);
            my $year = $create_time->year();
            #print "m $1 d $2 h $3 m $4 <$line>\n";

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
    #print Dump(\%info);
    return \%info;
}

sub truncate_timestamp {
    my $unix = shift;
    # mod 15*60
    # rem 2*60
    return int(($unix-120)/900)*900+120;
}


sub update_db {
    # param
    my $dbh = shift;
    my $info = shift;
    my $rv;
    my $ts = "";
    if (exists $$info{timestamp}) {
        $ts = $$info{timestamp};
    } else {
        $ts = truncate_timestamp($$info{create_time_unix});
    }
    $rv = $dbh->do("INSERT OR IGNORE INTO timestamp (timestamp) VALUES (?);", undef, $ts) or warn $dbh->errstr;
    #print "rv: $rv\n";
    for my $pair (@{$$info{border}}) {
        my ($rank, $score) = @$pair;
        $rv = $dbh->do("INSERT OR IGNORE INTO rank (timestamp, type, rank, score, viewer_id) VALUES (?, ?, ?, ?, ?);", undef, $ts, 1, $rank, $score, 0) or warn $dbh->errstr;
        #print "rv: $rv\n";
    }
}

main();
