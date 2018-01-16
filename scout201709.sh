set -x
RES_VER=10034500
MASTER=$(./get_master $RES_VER)
#echo $MASTER

DB="file:$MASTER?mode=ro"

# "SELECT * FROM gacha_data WHERE id == 30161;"
#"/* number of cards */" \
# "SELECT reward_id FROM gacha_available WHERE gacha_id == 30161;" 
#"SELECT gacha_available.reward_id, card_data.rarity, card_data.name FROM gacha_available INNER JOIN card_data ON card_data.id == gacha_available.reward_id WHERE gacha_available.gacha_id == 30161 AND card_data.rarity > 6;"
#"SELECT reward_id, card_data.name FROM gacha_available INNER JOIN card_data ON card_data.id = reward_id WHERE gacha_id = 30161 AND card_data.rarity > 6;"


# GACHA_ID: id in gacha_data
#    1xxxx: local audition gasha
#    2xxxx: platinum ticket, 10-ticket, ssr-ticket, sr-ticket
#    3xxxx: platinum audition gasha, (regular and limited)
#    4xxxx: daily once 60
#    7xxxx: ???


# 201709 scout ticket (ssr and sr)
GACHA_ID=30161

# 201801 scout ticket
GACHA_ID=30198

# 201801x sr-scout ticket


# type
#GACHA_ID=30166 # cute
#GACHA_ID=30167 
#GACHA_ID=30168


# id, rarity, name
sqlite3 "$DB" \
 ".schema gacha_data" \
 "SELECT id, source_guarantee, dicription FROM gacha_data ORDER BY id;" \
 "SELECT * FROM gacha_data WHERE id == $GACHA_ID;" \
 "SELECT count(*) FROM gacha_available WHERE gacha_id == $GACHA_ID;" \
 "/* all the card numbers */" \
 "SELECT count(*) FROM gacha_available INNER JOIN card_data ON id = reward_id WHERE gacha_id = $GACHA_ID AND rarity <= 4;" \
 "SELECT count(*) FROM gacha_available INNER JOIN card_data ON id = reward_id WHERE gacha_id = $GACHA_ID AND rarity < 7 AND rarity >= 5;" \
 "SELECT id, rarity, name FROM gacha_available INNER JOIN card_data ON id = reward_id WHERE gacha_id = $GACHA_ID AND rarity < 7 AND rarity >= 5;" \
 "SELECT count(*) FROM gacha_available INNER JOIN card_data ON id = reward_id WHERE gacha_id = $GACHA_ID AND rarity < 9 AND rarity >= 7;" \
 "SELECT id, rarity, name FROM gacha_available INNER JOIN card_data ON id = reward_id WHERE gacha_id = $GACHA_ID AND rarity < 9 AND rarity >= 7;"

