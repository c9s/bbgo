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
        declare -A order_params=()
        order_params[market]=$market
        order_params[side]=$side
        order_params[price]=$price
        order_params[volume]=$volume
        order_params[ord_type]="limit"
        submitOrder order_params
        ;;

    me)
        me
        ;;

    orders)
        market=$1
        declare -A orders_params=()
        orders_params[market]=$market
        myOrders orders_params
        ;;
        
    trades)
        market=$1
        declare -A trades_params=()
        trades_params[market]=$market
        myTrades trades_params
        ;;
esac
