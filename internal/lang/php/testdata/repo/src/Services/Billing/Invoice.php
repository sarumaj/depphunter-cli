<?php

namespace App\Services\Billing;

enum Invoice: string
{
    case Paid = 'paid';

    public function label(): string
    {
        return $this->value;
    }
}
