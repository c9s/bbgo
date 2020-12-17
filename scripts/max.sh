#!/bin/bash
source scripts/maxapi.sh


command=$1
shift

case "$command" in
    market)
        market=$1
        side=$2
        volume=$3

        if [[ $# < 3 ]] ; then
            echo "$0 market [market] [side] [volume]"
            exit
        fi

        declare -A order_params=()
        order_params[market]=$market
        order_params[side]=$side
        order_params[volume]=$volume
        order_params[ord_type]="market"
        submitOrder order_params
        ;;

    limit)
        market=$1
        side=$2
        price=$3
        volume=$4

        if [[ $# < 4 ]] ; then
            echo "$0 limit [market] [side] [price] [volume]"
            exit
        fi

        declare -A order_params=()
        order_params[market]=$market
        order_params[side]=$side
        order_params[price]=$price
        order_params[volume]=$volume
        order_params[ord_type]="limit"
        submitOrder order_params
        ;;

    me)
        me | jq -r '.accounts[] | select(.balance | tonumber > 0.0) | "\(.currency)\t\(.balance) \t(\(.locked) locked)"'
        ;;

    # open orders
    open)
        if [[ $# < 1 ]] ; then
            echo "$0 open [market]"
            exit
        fi

        market=$1
        declare -A orders_params=()
        orders_params[market]=$market
        myOrders orders_params | \
            jq -r '.[] | "\(.id) \(.market) \(.side) \(.ord_type) \(if .ord_type | test("stop") then "stop@" + .stop_price else "" end) price = \(if .ord_type | test("market") then "any" else .price end) \t volume = \(.volume) \(.state)"'
        ;;

    cancel)
        if [[ $# < 1 ]] ; then
            echo "$0 cancel [oid]"
            exit
        fi

        order_id=$1
        cancelOrder $order_id
        ;;
        
    trades)
        if [[ $# < 1 ]] ; then
            echo "$0 trades [market]"
            exit
        fi

        market=$1
        declare -A trades_params=()
        trades_params[market]=$market
        myTrades trades_params | \
            jq -r '.[] | "\(.id) \(.market) \(.side) \(.price) \t \(.volume) amount = \((.price | tonumber) * (.volume | tonumber) * 100 | floor / 100) \t fee = \( .fee // 0 | tonumber * 100000 | floor / 100000 ) \(.fee_currency)\t\( .created_at | strflocaltime("%Y-%m-%dT%H:%M:%S %Z") )"'
        ;;
esac
