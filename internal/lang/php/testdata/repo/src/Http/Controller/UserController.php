<?php

declare(strict_types=1);

namespace App\Http\Controller;

use App\Models\User;
use App\Models;
use App\Services\{Mailer, Billing\Invoice as Bill};
use Monolog\Logger;
use Psr\Log\LoggerInterface;
use GuzzleHttp\Client;
use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\Console\Command\Command;
use Carbon\Carbon;
use Acme\Tools\Formatter;
use Legacy_Mailer;
use Exception;
use function App\Support\format_money;
use function GuzzleHttp\Psr7\str;
use const App\Support\VERSION;

require_once __DIR__ . '/../../../bootstrap.php';
include 'config/app.php';
require dirname(__DIR__, 2) . '/helpers.php';
require $path;

final class UserController extends BaseController implements \JsonSerializable, Contracts\Handles
{
    public const LIMIT = 10;

    public function index(Request $request): array
    {
        $now = new \DateTimeImmutable();
        $client = new Client();
        \Monolog\Registry::getInstance('app');
        $widget = new \Unknown\Thing\Widget();
        try {
            \App\Models\User::find(1);
        } catch (\RuntimeException $e) {
        }
        return [\strlen('x'), \collect()];
    }

    private function helper(): void
    {
        $f = function () {
            function nested() {}
        };
    }
}
