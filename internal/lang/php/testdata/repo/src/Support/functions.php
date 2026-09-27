<?php

namespace App\Support;

const VERSION = '1.0';

function format_money(int $cents): string
{
    return number_format($cents / 100, 2);
}

if (!function_exists('App\Support\legacy')) {
    function legacy(): void
    {
    }
}

define('APP_DEBUG', false);
